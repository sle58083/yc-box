// Command server 是控制面入口：加载配置 → 初始化日志/加密 → 连库迁移 → 启动 gin。
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/singbox-panel/control-plane/internal/agent"
	"github.com/singbox-panel/control-plane/internal/api"
	"github.com/singbox-panel/control-plane/internal/audit"
	"github.com/singbox-panel/control-plane/internal/auth"
	"github.com/singbox-panel/control-plane/internal/config"
	"github.com/singbox-panel/control-plane/internal/configsvc"
	"github.com/singbox-panel/control-plane/internal/crypto"
	"github.com/singbox-panel/control-plane/internal/deploy"
	"github.com/singbox-panel/control-plane/internal/group"
	"github.com/singbox-panel/control-plane/internal/model"
	"github.com/singbox-panel/control-plane/internal/node"
	"github.com/singbox-panel/control-plane/internal/server"
	"github.com/singbox-panel/control-plane/internal/store"
	"github.com/singbox-panel/control-plane/internal/subscribe"
	"github.com/singbox-panel/control-plane/internal/traffic"
	"github.com/singbox-panel/control-plane/internal/user"
	"github.com/singbox-panel/control-plane/pkg/logger"
)

func main() {
	// 1. 加载配置
	cfg, err := config.Load("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "load config: %v\n", err)
		os.Exit(1)
	}

	// 2. 初始化结构化日志
	if err := logger.Init(cfg.Log.Level, cfg.Log.Format); err != nil {
		fmt.Fprintf(os.Stderr, "init logger: %v\n", err)
		os.Exit(1)
	}
	defer logger.Sync()
	log := logger.L()

	// 3. 初始化安全内核（AES-256-GCM 密钥；argon2id / 令牌哈希为包级函数）
	key, err := cfg.CryptoKey()
	if err != nil {
		log.Fatal("init crypto key failed", zap.Error(err))
	}
	km, err := crypto.NewStaticKeyManager(key)
	if err != nil {
		log.Fatal("init crypto key manager failed", zap.Error(err))
	}
	codec := crypto.New(km)

	// 4. 连库 + 迁移
	db, err := store.New(cfg.Database.ConnectionString(), cfg.Log.Level)
	if err != nil {
		log.Fatal("connect database failed", zap.Error(err))
	}
	if err := db.AutoMigrate(); err != nil {
		log.Fatal("run migration failed", zap.Error(err))
	}
	log.Info("database connected and migrated")

	// 5. 装配配置服务（T02：inbound/outbound/routing + 生成 + 校验 + 版本）。
	schemaDir := "singbox-schema"
	cfgSvc, err := configsvc.NewService(db, codec, schemaDir)
	if err != nil {
		log.Warn("schema validator disabled, fallback to no schema validation", zap.Error(err))
		cfgSvc, err = configsvc.NewService(db, codec, "")
		if err != nil {
			log.Fatal("init config service failed", zap.Error(err))
		}
	}

	// 6. 装配各领域服务（T04：认证/用户/订阅/流量/审计）。
	nodeSvc := node.NewService(db, codec)
	hub := agent.NewHub(log)
	orch := deploy.NewOrchestrator(cfgSvc, hub, log, 30*time.Second)

	authSvc := auth.NewService(db, cfg.JWT)
	userSvc := user.NewService(db, codec)
	subSvc := subscribe.NewService(db, userSvc, codec)
	trafficSvc := traffic.NewService(db, codec)
	auditSvc := audit.NewService(db)
	groupSvc := group.NewService(db)

	// 幂等初始化内置角色与默认分组。
	if err := userSvc.SeedRoles(context.Background()); err != nil {
		log.Fatal("seed roles failed", zap.Error(err))
	}
	created, err := userSvc.SeedAdmin(context.Background(), cfg.Bootstrap.AdminUsername, cfg.Bootstrap.AdminPassword)
	if err != nil {
		log.Fatal("seed admin failed", zap.Error(err))
	}
	if created {
		log.Info("initial admin created, please login and change the initial password",
			zap.String("username", cfg.Bootstrap.AdminUsername))
	}
	if _, err := groupSvc.EnsureDefaultGroup(context.Background()); err != nil {
		log.Fatal("seed default group failed", zap.Error(err))
	}

	agentHandler := agent.NewHandler(nodeSvc, hub, log, orch.NotifyApplyResult, func(nodeID uuid.UUID, data agent.TrafficReportData) {
		records := make([]traffic.Record, 0, len(data.Records))
		for _, r := range data.Records {
			records = append(records, traffic.Record{
				TS:          r.TS,
				ProxyUserID: r.ProxyUserID,
				InboundTag:  r.InboundTag,
				OutboundTag: r.OutboundTag,
				UpBytes:     r.UpBytes,
				DownBytes:   r.DownBytes,
			})
		}
		if err := trafficSvc.Ingest(context.Background(), nodeID, records); err != nil {
			log.Warn("ingest traffic failed", zap.Error(err))
		}
	})
	apiHandler := api.NewHandler(cfgSvc, nodeSvc, orch, agentHandler, authSvc, userSvc, subSvc, trafficSvc, auditSvc, groupSvc)

	// 7. 启动 HTTP（业务路由 + WS + 健康检查）。
	gin.SetMode(cfg.Server.Mode)
	router := server.NewRouter(apiHandler, authSvc, auditSvc)

	router.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, model.OK(gin.H{"status": "ok", "time": time.Now().UTC()}))
	})
	router.GET("/api/v1/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, model.OK(gin.H{"time": time.Now().UTC()}))
	})

	srv := &http.Server{
		Addr:              cfg.Server.Addr(),
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("control plane listening", zap.String("addr", cfg.Server.Addr()))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("http server error", zap.Error(err))
		}
	}()

	// 8. 优雅退出
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("shutting down control plane")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Error("graceful shutdown failed", zap.Error(err))
	}
	log.Info("control plane stopped")
}
