# sing-box 面板 — 协议支持矩阵 + 渲染器扩展点设计

> 架构师：高见远（Gao）
> 面向对象：工程师（增量设计，配合 `docs/system_design.md` 使用）
> 定位：把「中立配置模型 → sing-box 原生 JSON」的翻译层，从「if/switch 硬编码」升级为「协议渲染器插件化」，实现**新增协议不改数据模型、不改内核、不改 Generator 主体**。

---

## 0. 一句话结论

- 协议能力全部收敛为 **`ProtocolRenderer` 接口**：一个协议 = 一个渲染器实现（渲染 + 校验 + 订阅 URI + 元数据）。
- Generator 只做「查表分发 + 全局结构拼装」，**不再感知具体协议字段**。
- 协议专属字段（如 anytls 的会话池三件套、padding_scheme）**全部落 `settings/tls/transport` 的 JSONB**，中立模型零改动。
- 新增 anytls 的完整路径 = 新增 `renderer/anytls.go` + 注册 + 前端元数据 + schema 片段 + 单测，**约 12~14 文件 / ~1000~1400 行 / 1~1.5 人日**。

---

## 1. 协议支持矩阵

> 图例：方向 `B`=双向（inbound+outbound）、`I`=仅入站、`O`=仅出站；`⚠`=以 sing-box 官方文档为准，落地前需对照目标内核版本复核。

### 1.1 协议能力矩阵（主表）

| 维度 \ 协议 | shadowsocks | shadowsocks2022 | vmess | vless | trojan | hysteria | hysteria2 | tuic | **anytls** | shadowtls | wireguard | naive | ssh |
|---|---|---|---|---|---|---|---|---|---|---|---|---|---|
| **支持方向** | B | B | B | B | B | B | B | B | **B** | B⚠ | B | B | B |
| **MVP 优先级** | P0 | P1 | P0 | P0 | P0 | P2 | P0 | P0 | **P1** | P2 | P2 | P2 | P2 |
| **认证字段类型** | method+明文密码 | method+base64 PSK（单用户） | UUID(v4/v5) | UUID(+flow) | 明文密码（内部 SHA224） | 明文 auth 串 | 明文密码 / user:pass | UUID+明文密码 | **base64 密码** | 明文握手密码⚠ | 密钥对(base64 公私钥+PSK) | user+password(HTTP Basic) | user+password 或公钥 |
| **传输方式** | TCP（可套插件） | TCP（TCP/UDP 复用） | TCP/WS/gRPC/HTTPUpgrade | TCP/WS/gRPC/HTTPUpgrade | TCP/WS/gRPC/HTTPUpgrade | QUIC（内建） | QUIC（内建，h2 混淆可选） | QUIC（内建） | **TCP（内建会话复用，不可套 WS/gRPC）** | 自定义（TLS 混淆，包裹 TCP 协议） | UDP（内建） | h2（HTTP/2） | TCP（SSH 协议） |
| **TLS 能力** | 无原生 TLS | 无原生 TLS | 传输层 TLS（WS/gRPC 内），无 Reality | 普通 TLS + Reality + ECH | 必选普通 TLS（自带握手），无 Reality | 内置 QUIC TLS | 内置 QUIC TLS（支持自签） | 内置 QUIC TLS | **必选普通 TLS，不支持 Reality/ECH** | ShadowTLS 握手（替代 TLS） | 自有加密（无 TLS） | 必选 TLS + h2 | 自有加密（无 TLS） |
| **多用户支持** | 入站 users[]（method+password） | 单 PSK（每入站一个） | users[]（uuid） | users[]（uuid+flow） | users[]（password） | 单 auth | users[]（password/userpass） | users[]（uuid+password） | **users[]（name+password）** | N/A（随底层协议） | peers[] | users[]（user+password） | users[]（user+password/公钥） |
| **UDP / 多路复用** | UDP 支持；multiplex+udp_over_tcp | TCP+UDP 单连接复用 | UDP over TCP | UDP over TCP | UDP over TCP | 原生 UDP（QUIC） | TCP+UDP（QUIC） | TCP+UDP（QUIC） | **TCP（UDP 复用以文档为准）⚠** | 随底层协议⚠ | UDP only | TCP（h2） | TCP（UDP over TCP 可选）⚠ |
| **订阅 URI scheme** | `ss://base64(method:pass)@host:port` | `ss://`（2022-blake3+base64 PSK） | `vmess://base64(JSON)` | `vless://uuid@host:port?…` | `trojan://pass@host:port?…` | `hysteria://host:port?auth=…&protocol=udp` | `hysteria2://[user:]pass@host:port?…` | `tuic://uuid:pass@host:port?…` | **`anytls://pass@host:port?sni=…`** | 无标准（随底层 ss:// + plugin 扩展）⚠ | `wireguard://base64(wg-quick)` | `naive+https://user:pass@host:port` | N/A（无标准订阅格式） |

### 1.2 协议 × TLS 组合合法性约束

| 协议 | 允许的 TLS/加密组合 | **禁止组合** | 说明 |
|---|---|---|---|
| vless | 无 TLS / 普通 TLS / Reality / ECH | — | **Reality 仅 vless**（本面板 MVP 内）；ECH 需目标内核 ≥ 对应版本 |
| vmess | 无 TLS / 传输层 TLS（ws/grpc/httpupgrade 内 `tls`） | Reality / ECH | vmess 无 native `tls` 字段，TLS 走 transport 层 |
| trojan | 普通 TLS（**必选**） | Reality / ECH | 协议自带 TLS 握手（password 参与握手派生） |
| **anytls** | 普通 TLS（**必选**） | **Reality / ECH** | AnyTLS 自带 TLS 握手 + `padding_scheme`；与 Reality 定位冲突 |
| tuic | 内置 QUIC TLS | 嵌套外层 TLS/transport / Reality | 自带 QUIC，二次包裹 TLS 非法 |
| hysteria2 | 内置 QUIC TLS（自签可选） | 嵌套 TLS / Reality | 自带 QUIC |
| hysteria | 内置 QUIC TLS | 嵌套 TLS | v1，P2 |
| ss / ss2022 | 无（或 SIP003 插件） | 注入 native `tls` 字段 | 无原生 TLS，靠 transport/插件 |
| shadowtls | ShadowTLS 握手 | Reality / ECH（**互斥**） | ShadowTLS/Reality/ECH 三者都是 TLS 混淆方案，只能二选一 |
| wireguard / naive / ssh | 自带协议级加密 | 嵌套 tls/transport | 协议级加密，禁止外层再加 TLS |

> **组合约束落点**：以上规则全部下沉到各渲染器的 `ValidateInbound/ValidateOutbound`（保存前强校验），Generator 与前端只消费渲染器给出的「允许清单」，不重复硬编码规则（详见 §4.3）。

> **⚠️ Reality 环境依赖（重要）**：Reality 依赖「服务端向握手目标（handshake server）发起真实 TLS 握手并伪装」这一特定网络行为。真机实测发现，**部分 VPS（尤其对 TLS 流量特征敏感的海外节点，如某些华为云国际节点）会被运营商 DPI/中间设备干扰**，导致 `REALITY: processed invalid connection` / 客户端 `reality verification failed`，即使配置完全正确（密钥对匹配、short_id 一致、时间同步、同机 127.0.0.1 测试）也无法建立连接。这是**环境限制，非面板/代码问题**。建议：① 生产环境优先用**普通 TLS**（证书方式，稳定可靠）；② 使用 Reality 前先在目标 VPS 上验证握手目标可达且不被干扰；③ 前端已保留 Reality 配置项，但需提示用户「部分环境不可用」。

---

## 2. 渲染器扩展点设计

### 2.1 目录变化（`configsvc` 下新增 `renderer` 子包）

```
internal/configsvc/
├── entity.go               # 中立模型（不变）
├── generator.go            # 只做「查表分发 + 全局结构拼装」（改造）
├── template.go             # 全局结构模板（log/dns/route/experimental）（收窄职责）
├── validate.go             # 通用业务校验（tag 唯一/端口/引用），委托协议校验给 renderer
├── schema.go               # 生成后 JSON Schema 校验（按 type 组装协议 schema 片段）
├── version.go              # 版本管理（不变）
└── renderer/               # ★ 新增：协议渲染器插件包
    ├── renderer.go         # ProtocolRenderer / RenderContext / ProxyUser / 共享入口
    ├── registry.go         # protocol.Register / Get / List（init() + map）
    ├── tls.go              # 共享 TLS 渲染（普通 TLS / Reality / ECH）+ 组合校验
    ├── transport.go        # 共享传输层渲染（tcp/ws/grpc/httpupgrade）
    ├── common.go           # 通用字段渲染（tag/listen/port/server/server_port）+ base64 工具
    ├── anytls.go           # ★ anytls 渲染器（inbound+outbound+校验+订阅 URI）+ init() 注册
    └── *_test.go           # 各渲染器单测
```

### 2.2 渲染器接口（Go）

> 说明：主理人示例里的 `Validate(cfg InboundConfig | OutboundConfig)` 是「联合类型」，Go 没有该语法；且入站/出站必填字段不同，因此拆成 `ValidateInbound` / `ValidateOutbound` 两个方法，更符合 Go 习惯、校验也更精准。

```go
package renderer

import (
    "context"
    "encoding/json"

    "your.module/internal/model"
)

// ProxyUser 是渲染时注入的代理用户视图（敏感值已由 configsvc 解密）。
// 渲染器负责「编码规则」（如 anytls 要 base64），但不得负责「解密」。
type ProxyUser struct {
    ID       string
    Name     string
    UUID     string // vless/vmess 用，已解密
    Password string // 协议密码明文（已解密；anytls 渲染器会再 base64）
}

// RenderContext 提供渲染期间共享数据与已解密的敏感值。
type RenderContext struct {
    Ctx   context.Context
    Users []ProxyUser // 该分组 active 用户（is_user_inbound 时注入）
}

// ProtocolRenderer 协议渲染器接口：一个协议 = 一个实现。
type ProtocolRenderer interface {
    // Protocol 返回协议标识，必须与 inbound_config.type / outbound_config.type 一致。
    Protocol() string

    // RenderInbound 渲染 sing-box inbounds[] 的一个元素。
    // users 仅在 cfg.IsUserInbound 为真时非空。
    RenderInbound(ctx *RenderContext, cfg model.InboundConfig, users []ProxyUser) (map[string]any, error)

    // RenderOutbound 渲染 sing-box outbounds[] 的一个元素。
    RenderOutbound(ctx *RenderContext, cfg model.OutboundConfig) (map[string]any, error)

    // ValidateInbound / ValidateOutbound：保存前业务校验（含协议×TLS 组合约束）。
    ValidateInbound(cfg model.InboundConfig) error
    ValidateOutbound(cfg model.OutboundConfig) error

    // SubscriptionURI 生成订阅 URI；协议无标准订阅格式时返回 ErrUnsupported。
    SubscriptionURI(cfg model.OutboundConfig, user ProxyUser) (string, error)

    // Meta 返回前端动态表单所需的协议元数据（方向、字段 schema、允许的 TLS 组合）。
    Meta() ProtocolMeta
}

// ProtocolMeta 协议元数据，前端据此渲染动态表单与约束。
type ProtocolMeta struct {
    Protocol    string
    Directions  []Direction            // inbound / outbound
    Inbound     []FieldSchema          // 入站专属字段描述
    Outbound    []FieldSchema          // 出站专属字段描述
    TLSAllowed  []TLSMode              // 允许的 TLS 组合，如 {"plain"} 或 {"plain","reality","ech"}
    TLSRequired bool                   // 是否强制 TLS（anytls/trojan 为 true）
    MultiUser   bool                   // 是否支持 users[] 数组
}

type Direction string
const (
    DirectionInbound  Direction = "inbound"
    DirectionOutbound Direction = "outbound"
)

type TLSMode string
const (
    TLSModeNone    TLSMode = "none"
    TLSModePlain   TLSMode = "plain"   // 普通 TLS
    TLSModeReality TLSMode = "reality"
    TLSModeECH     TLSMode = "ech"
)

// FieldSchema 是前端动态表单的字段描述（JSON-schema 风格，最小子集）。
type FieldSchema struct {
    Key      string          `json:"key"`      // settings 里的字段名
    Label    string          `json:"label"`
    Type     string          `json:"type"`     // text/number/bool/select/array/duration
    Required bool            `json:"required"`
    Encrypt  bool            `json:"encrypt"`  // 是否 🔐 加密落库（如 anytls password）
    Options  []FieldOption   `json:"options,omitempty"`
    Default  json.RawMessage `json:"default,omitempty"`
}

type FieldOption struct {
    Label string `json:"label"`
    Value string `json:"value"`
}
```

### 2.3 注册机制（`init()` + registry map）

```go
package renderer

import (
    "fmt"
    "sort"
    "sync"
)

var (
    registryMu sync.RWMutex
    registry   = map[string]ProtocolRenderer{}
)

// Register 由各渲染器在 init() 中调用；重复注册同协议直接 panic（编译期/启动期暴露错误）。
func Register(r ProtocolRenderer) {
    registryMu.Lock()
    defer registryMu.Unlock()
    p := r.Protocol()
    if _, dup := registry[p]; dup {
        panic(fmt.Sprintf("renderer: protocol %q already registered", p))
    }
    registry[p] = r
}

// Get 按 type 查表；Generator 只依赖此函数，绝不硬编码协议名。
func Get(proto string) (ProtocolRenderer, bool) {
    registryMu.RLock()
    defer registryMu.RUnlock()
    r, ok := registry[proto]
    return r, ok
}

// List 返回已注册协议（排序，供前端/校验白名单使用）。
func List() []string {
    registryMu.RLock()
    defer registryMu.RUnlock()
    out := make([]string, 0, len(registry))
    for p := range registry {
        out = append(out, p)
    }
    sort.Strings(out)
    return out
}
```

`anytls.go` 中注册：

```go
func init() {
    Register(&AnyTLSRenderer{})
}
```

### 2.4 Generator / template 中的挂载点与调用链

- `generator.go`：原来是「聚合上下文 → 硬编码 if/switch 渲染」；改造后只保留**两段循环**：
  1. 遍历 `ctx.Inbounds` → `r, ok := renderer.Get(cfg.Type)` → `r.RenderInbound(...)`
  2. 遍历 `ctx.Outbounds` → `renderer.Get(cfg.Type)` → `r.RenderOutbound(...)`
  查不到（未注册协议）→ 返回明确的 `ErrUnknownProtocol`。
- `template.go`：**收窄为「全局结构拼装」**——`log` / `dns` / `route` / `experimental` / 顶层骨架（`inbounds`/`outbounds` 数组的容器），不再承载任何协议字段映射。
- `validate.go`：通用校验（tag 唯一、端口冲突、引用完整性、域名/IP 正则）保持不变，在保存 `inbound/outbound` 时**追加一步** `renderer.Get(type).ValidateInbound/Outbound(...)`。
- `schema.go`：生成后校验按 `type` 组装 `singbox-schema/` 下的协议片段进行 `gojsonschema` 校验。

```mermaid
sequenceDiagram
    participant API as API/Handler
    participant SV as ConfigService
    participant GEN as Generator
    participant REG as renderer.Registry
    participant R as AnyTLSRenderer
    participant SH as shared(tls/transport)
    participant SC as SchemaValidator

    API->>SV: Generate(nodeID)
    SV->>SV: 聚合 NodeConfigContext + 解密 ProxyUsers
    SV->>GEN: Generate(ctx)
    loop 每个 inbound
        GEN->>REG: Get(cfg.Type)
        REG-->>GEN: renderer
        GEN->>R: RenderInbound(ctx, cfg, users)
        R->>R: base64(password) + padding_scheme
        R->>SH: RenderTLS(cfg.TLS, PlainOnly)
        SH-->>R: tls map
        R-->>GEN: inbounds[i] map
    end
    loop 每个 outbound
        GEN->>REG: Get(cfg.Type)
        REG-->>GEN: renderer
        GEN->>R: RenderOutbound(ctx, cfg)
        R->>R: base64(password) + 会话池三件套 + client_metadata
        R->>SH: RenderTLS(cfg.TLS, PlainOnly)
        SH-->>R: tls map
        R-->>GEN: outbounds[i] map
    end
    GEN->>GEN: 拼装 log/dns/route 全局结构
    GEN->>SC: Validate(generatedJSON)
    SC-->>GEN: ok
    GEN-->>SV: sing-box JSON
```

---

## 3. 新增 anytls 的完整落地案例（重点）

> 目标：从零让面板支持 anytls（sing-box ≥ 1.12.0），复用既有渲染器框架，不改数据模型、不改内核。

### 3.1 关键语义差异（必须先理解）

| 字段 | anytls 语义 | 与其他协议的差异 |
|---|---|---|
| `password` | **base64 编码**（非明文） | vmess/vless 用 UUID；ss 用明文 password；trojan 用明文；**只有 anytls 要求 base64** |
| `padding_scheme` | **仅 inbound**，数组，用于流量填充混淆 | 其他协议无此字段 |
| `idle_session_check_interval` / `idle_session_timeout` / `min_idle_session` | **仅 outbound**，会话池三件套 | 其他协议无会话池概念 |
| `client_metadata` | 仅 outbound，自 sing-box 1.13.16 起 | 可选字符串 |
| TLS | inbound+outbound 都**必选**普通 TLS | vless 可无 TLS/Reality；tuic 自带 QUIC |
| 客户端兼容 | mihomo/Clash.Meta 内置、Shadowrocket 2.2.65+ | mihomo **不支持 AnyTLS+Reality** |

### 3.2 文件清单与改动要点

| # | 文件 | 新增/修改 | 改动要点 | 是否必须 |
|---|---|---|---|---|
| 1 | `internal/configsvc/renderer/anytls.go` | **新增** | AnyTLSRenderer：inbound（users base64 密码 + padding_scheme + tls）、outbound（server/port/password base64 + 会话池三件套 + client_metadata + tls）、Validate、SubscriptionURI、Meta、`init()` 注册 | ✅ 必须 |
| 2 | `internal/configsvc/renderer/anytls_test.go` | **新增** | 渲染正确性 / base64 编码 / 组合约束 / 会话池字段 | ✅ 必须 |
| 3 | `internal/configsvc/renderer/renderer.go` | **新增** | 接口 + RenderContext + ProxyUser + Meta/FieldSchema | ✅ 必须（框架一次性建设） |
| 4 | `internal/configsvc/renderer/registry.go` | **新增** | Register/Get/List | ✅ 必须（框架） |
| 5 | `internal/configsvc/renderer/tls.go` | **新增** | 共享 TLS 渲染 + 组合校验（anytls 只允许 plain） | ✅ 必须（共享组件） |
| 6 | `internal/configsvc/renderer/transport.go` | **新增** | 共享传输层渲染（anytls 无 transport，直接复用「无 transport」分支） | ✅ 必须（共享组件） |
| 7 | `internal/configsvc/renderer/common.go` | **新增** | tag/listen/port/server/server_port 通用字段 + `base64Encode` | ✅ 必须（共享组件） |
| 8 | `internal/configsvc/generator.go` | **修改** | 去掉硬编码协议 switch，改为 `renderer.Get(type)` 查表分发 | ✅ 必须 |
| 9 | `internal/configsvc/template.go` | **修改** | 收窄为全局结构拼装；协议字段映射迁移到 renderer | ✅ 必须（框架改造） |
| 10 | `internal/configsvc/validate.go` | **修改** | 保存前追加 `renderer.ValidateInbound/Outbound` 委托 | ✅ 必须 |
| 11 | `internal/model/constants.go` | **修改** | 协议类型白名单加 `"anytls"`（若白名单为常量而非 DB CHECK） | ✅ 必须 |
| 12 | `singbox-schema/inbound-anytls.json` + `outbound-anytls.json` | **新增** | 第二层 gojsonschema 校验用的 anytls 协议片段 | ⚠ 见 §3.8 |
| 13 | `web/src/utils/protocols.ts` | **新增/修改** | 协议枚举 + Meta（方向/TLS 组合/动态字段 schema） | ✅ 必须（前端表单） |
| 14 | `web/src/components/ProtocolForm.tsx` | **新增** | 通用动态表单渲染器（按 FieldSchema 渲染） | ✅ 必须（一次性建设） |
| 15 | `web/src/pages/inbounds/*`、`web/src/pages/outbounds/*` | **修改** | 协议下拉加 anytls；接入 ProtocolForm | ✅ 必须 |
| 16 | `docs/protocol-matrix-renderer.md` | **新增** | 本文档 | ✅ 必须 |
| 17 | `migrations/*.sql` | **可选** | 若想给 `type` 加 `CHECK` 约束才需迁移；**MVP 建议不加 DB CHECK，用代码白名单** | ❌ 可选 |

### 3.3 中立模型是否需要动？

**不需要**。`inbound_config.settings/tls/transport` 与 `outbound_config.settings` 都是通用 JSONB，anytls 的所有专属字段（base64 password、padding_scheme、会话池三件套、client_metadata）**全部塞进现有 JSONB**：

- anytls inbound：`tls`（普通 TLS）→ `inbound_config.tls`；`padding_scheme` → `inbound_config.settings`
- anytls outbound：`server/server_port/password/会话池三件套/client_metadata` → `outbound_config.settings`；`tls` → 见 §3.9 的对称性修正

**唯一要改的**是**代码级常量**（`internal/model/constants.go` 的允许类型白名单加 `"anytls"`），以及前端协议枚举。DB 层 `type varchar(32)` 无约束，零迁移。

### 3.4 `renderer/anytls.go` 核心逻辑（伪代码级）

```go
package renderer

import (
    "encoding/base64"
    "errors"
    "fmt"

    "your.module/internal/model"
)

type AnyTLSRenderer struct{}

func (r *AnyTLSRenderer) Protocol() string { return "anytls" }

func (r *AnyTLSRenderer) RenderInbound(ctx *RenderContext, cfg model.InboundConfig, users []ProxyUser) (map[string]any, error) {
    out := map[string]any{
        "type": "anytls", "tag": cfg.Name,
        "listen": cfg.Listen, "listen_port": cfg.Port,
    }

    // users：密码必须 base64（渲染器负责编码，不再依赖前端存 base64）
    us := make([]map[string]any, 0, len(users))
    for _, u := range users {
        us = append(us, map[string]any{
            "name":     u.Name,
            "password": base64.StdEncoding.EncodeToString([]byte(u.Password)),
        })
    }
    out["users"] = us

    // tls：inbound 必选普通 TLS（证书/key），强制 plain，禁止 reality/ech
    tlsMap, err := RenderTLSPlain(cfg.TLS, true /*requireCert*/)
    if err != nil {
        return nil, err
    }
    out["tls"] = tlsMap

    // padding_scheme：仅 inbound，来自 settings JSONB
    if ps, ok := cfg.Settings["padding_scheme"]; ok {
        out["padding_scheme"] = ps
    }
    return out, nil
}

func (r *AnyTLSRenderer) RenderOutbound(ctx *RenderContext, cfg model.OutboundConfig) (map[string]any, error) {
    out := map[string]any{
        "type": "anytls", "tag": cfg.Name,
        "server":      cfg.Settings["server"],
        "server_port": cfg.Settings["server_port"],
        // ★ 关键：outbound 密码也是 base64
        "password": base64.StdEncoding.EncodeToString([]byte(str(cfg.Settings["password"]))),
    }

    // 会话池三件套 + client_metadata（仅 outbound，可选，存在才渲染）
    for _, k := range []string{
        "idle_session_check_interval", // "30s"
        "idle_session_timeout",        // "30s"
        "min_idle_session",            // 5
        "client_metadata",             // ""
    } {
        if v, ok := cfg.Settings[k]; ok {
            out[k] = v
        }
    }

    tlsMap, err := RenderTLSPlain(cfg.TLS, false /*requireCert，需 server_name*/)
    if err != nil {
        return nil, err
    }
    out["tls"] = tlsMap
    return out, nil
}

func (r *AnyTLSRenderer) ValidateInbound(cfg model.InboundConfig) error {
    if cfg.TLS == nil || !tlsEnabled(cfg.TLS) {
        return errors.New("anytls inbound: tls.enabled 必须为 true")
    }
    if _, ok := cfg.TLS["certificate_path"]; !ok {
        return errors.New("anytls inbound: 缺少 certificate_path")
    }
    if _, ok := cfg.TLS["key_path"]; !ok {
        return errors.New("anytls inbound: 缺少 key_path")
    }
    if realityEnabled(cfg.TLS) || echEnabled(cfg.TLS) {
        return errors.New("anytls inbound: 不支持 Reality/ECH")
    }
    // users 非空、每个 name+password 非空、password 为合法 base64
    return validateAnyTLSUsers(cfg.Settings)
}

func (r *AnyTLSRenderer) ValidateOutbound(cfg model.OutboundConfig) error {
    if empty(cfg.Settings["server"]) || empty(cfg.Settings["server_port"]) {
        return errors.New("anytls outbound: server/server_port 必填")
    }
    if empty(cfg.Settings["password"]) {
        return errors.New("anytls outbound: password 必填")
    }
    if cfg.TLS == nil || !tlsEnabled(cfg.TLS) {
        return errors.New("anytls outbound: tls.enabled 必须为 true")
    }
    if empty(cfg.TLS["server_name"]) {
        return errors.New("anytls outbound: 缺少 tls.server_name")
    }
    if realityEnabled(cfg.TLS) || echEnabled(cfg.TLS) {
        return errors.New("anytls outbound: 不支持 Reality/ECH（客户端 mihomo 也不支持 AnyTLS+Reality）")
    }
    // 会话池字段校验：interval/timeout 为正 duration，min_idle_session ≥ 0
    return validateAnyTLSSessionPool(cfg.Settings)
}

func (r *AnyTLSRenderer) SubscriptionURI(cfg model.OutboundConfig, user ProxyUser) (string, error) {
    // anytls://<base64-password>@<server>:<port>?sni=<server_name>#<tag>
    // 密码在 URI 中直接带 base64 形式；具体以 mihomo/sing-box 订阅解析为准。
    pwd := base64.StdEncoding.EncodeToString([]byte(user.Password))
    sni := str(cfg.TLS["server_name"])
    return fmt.Sprintf("anytls://%s@%s:%s?sni=%s#%s",
        pwd, str(cfg.Settings["server"]), str(cfg.Settings["server_port"]), sni, cfg.Name), nil
}

func (r *AnyTLSRenderer) Meta() ProtocolMeta {
    return ProtocolMeta{
        Protocol:    "anytls",
        Directions:  []Direction{DirectionInbound, DirectionOutbound},
        TLSAllowed:  []TLSMode{TLSModePlain}, // 仅普通 TLS
        TLSRequired: true,
        MultiUser:   true,
        Inbound: []FieldSchema{
            {Key: "padding_scheme", Label: "Padding 方案", Type: "array", Required: false},
        },
        Outbound: []FieldSchema{
            {Key: "server", Label: "服务器地址", Type: "text", Required: true},
            {Key: "server_port", Label: "端口", Type: "number", Required: true},
            {Key: "password", Label: "密码", Type: "text", Required: true, Encrypt: true}, // 明文入、渲染时 base64
            {Key: "idle_session_check_interval", Label: "会话检查间隔", Type: "duration", Required: false},
            {Key: "idle_session_timeout", Label: "会话空闲超时", Type: "duration", Required: false},
            {Key: "min_idle_session", Label: "最小空闲会话数", Type: "number", Required: false},
            {Key: "client_metadata", Label: "Client Metadata", Type: "text", Required: false},
        },
    }
}

func init() { Register(&AnyTLSRenderer{}) }
```

### 3.5 校验逻辑（Validate）清单

| 校验项 | inbound | outbound |
|---|---|---|
| `tls.enabled == true`（anytls 强制 TLS） | ✅ | ✅ |
| inbound 必须有 `certificate_path` + `key_path` | ✅ | — |
| outbound 必须有 `tls.server_name` | — | ✅ |
| 禁止 `reality` / `ech` | ✅ | ✅ |
| `password` 非空 | ✅（users[].password） | ✅ |
| `password` 为合法 base64（或可编码为 base64） | ✅ | ✅ |
| `server` / `server_port` 非空 | — | ✅ |
| 会话池三件套类型/范围（duration 合法、`min_idle_session ≥ 0`） | — | ✅ |
| `padding_scheme` 为数组 | ✅ | — |

### 3.6 订阅 URI 生成

```
anytls://<base64-password>@<server>:<server_port>?sni=<server_name>#<tag>
```

- 密码取**渲染后的 base64 形式**（与 `RenderOutbound` 一致，保证「订阅拿到的密码 = 配置里的密码」）。
- 参数扩展（`insecure`、`idle_*`）按客户端支持增量补充；**首版先覆盖最小可用 URI**，其余交给 `?param` 透传，具体以 mihomo/sing-box 订阅解析为准。
- 若目标客户端尚不支持标准 `anytls://`，可在 `SubscriptionURI` 返回 `ErrUnsupported`，订阅模块自动降级为「仅下发完整 JSON 配置」，不阻塞主流程。

### 3.7 前端表单

- `web/src/utils/protocols.ts`：维护协议枚举 + 每个协议的 `Meta`（方向、TLS 组合白名单、字段 schema）。**协议元数据由后端 `Meta()` 经 `GET /protocols` 下发**（推荐），避免前后端各维护一份、出现漂移。
- `web/src/components/ProtocolForm.tsx`：通用动态表单——按 `FieldSchema[]` 渲染文本框/数字/开关/下拉/数组/duration，`Encrypt=true` 的字段入库前走加密接口。
- `web/src/pages/inbounds/*`、`outbounds/*`：协议下拉加 `anytls`，选中后渲染专属字段（inbound：`padding_scheme` + TLS 证书；outbound：`server/port/password/会话池三件套/client_metadata` + TLS server_name）。
- 前端只做「体验层约束」（非法 TLS 组合直接置灰/隐藏），**唯一权威仍是后端渲染器 Validate**（前端可被绕过）。

### 3.8 sing-box JSON Schema 校验

- 设计里第二层校验用 `gojsonschema`，schema 文件由面板**自维护**（`singbox-schema/`，随内核版本演进，sing-box 无单一官方 schema 文件）。
- anytls 需要**新增** `inbound-anytls.json` / `outbound-anytls.json` 两个片段，`schema.go` 按 `type` 组装后校验。
- **务实建议**：第二层是「快速失败」的便捷层，**最终权威是第三层 Agent 的 `sing-box check -c`**。若短期内 schema 片段维护成本高，可先让 anytls 跳过第二层、仅靠第三层兜底（但会失去「生成阶段就拦截」的能力，不推荐长期如此）。

### 3.9 对既有模型的对称性修正（重要发现，与 anytls 无强绑定）

`system_design.md §4.2` 里 `outbound_config` 只有 `settings` JSONB，**没有独立的 `tls` / `transport` 列**（而 `inbound_config` 有）。这会导致 vless/trojan/anytls 出站的 TLS、transport 无处安放、只能混进 `settings`。

**建议（P0，纳入 T02）**：给 `outbound_config` 增加 `tls jsonb`、`transport jsonb` 两列（nullable，向后兼容迁移），与 `inbound_config` 对齐。这样：
- 共享组件 `tls.go` / `transport.go` 可同时服务 inbound 与 outbound，真正复用；
- anytls 出站 TLS 自然落到 `outbound_config.tls`，语义清晰。

> 若不采纳，anytls 也能工作（出站 TLS 放 `settings`），但 vless/trojan 出站的 transport 会被迫挤压进 `settings`，长期混乱。**强烈建议对齐。**

### 3.10 工作量估算

| 模块 | 文件数 | 代码量（估） | 说明 |
|---|---|---|---|
| 后端渲染器框架（接口/registry/tls/transport/common） | 5 | ~300-450 行 Go | 一次性建设，后续协议复用 |
| anytls 渲染器 + 单测 | 2 | ~250-350 行 Go | 本文核心增量 |
| generator/template/validate 改造 | 3 | ~150-250 行 Go | 去掉硬编码、加委托 |
| 常量 + schema 片段 | 3 | ~80-120 行 | 白名单 + 2 个 schema |
| 前端（protocols.ts / ProtocolForm / 页面接入） | 3 | ~250-400 行 TS | 元数据 + 动态表单 |
| 文档 | 1 | — | 本文档 |
| **合计** | **约 12~14 文件** | **约 1000~1400 行** | **约 1~1.5 人日（含联调+单测）** |

### 3.11 复用 vs 必写（边界）

**✅ 复用（不为 anytls 重写，共享组件/框架）**：
- `tag/listen/listen_port/server/server_port` 通用字段渲染（`common.go`）
- 普通 TLS 渲染 + 组合校验（`tls.go`，anytls 只调 `RenderTLSPlain`）
- 传输层渲染（`transport.go`，anytls 命中「无 transport」分支）
- registry 分发（`registry.go`）
- Generator 三层校验框架、敏感信息加密链路（`crypto` 包）
- 前端动态表单框架（`ProtocolForm`）、订阅 token/下发链路

**⚠️ 必写（anytls 专属）**：
- base64 密码编码（与 vmess UUID / ss 明文密码的语义差异）
- `padding_scheme`（inbound）、会话池三件套 + `client_metadata`（outbound）
- anytls 组合校验规则（必选 TLS、禁 Reality/ECH）
- `anytls://` 订阅 URI
- 前端 anytls 字段 schema
- anytls 单测

---

## 4. 设计要点总结

### 4.1 为什么「协议细节存 JSONB + 渲染器插件化」能不改数据模型、不改内核

1. **数据模型只存「结构骨架」**：`type/name/listen/port/settings/tls/transport` 是与协议无关的通用容器。任何协议的专属字段都塞进 `settings`（或 `tls`/`transport`）JSONB，因此新增协议**零迁移**。
2. **翻译逻辑按协议隔离**：每个协议的「JSONB → sing-box JSON」翻译封装在独立渲染器里，`init()` 自注册。Generator 通过 `type` 查表分发，**新增协议 = 新增一个文件 + 一次注册**，不触碰 Generator 主体与内核。
3. **内核完全透明**：面板产出的是标准 sing-box JSON，内核版本演进（如 anytls 在 1.12.0 内置）只影响「渲染器字段 + schema 片段」，与数据模型无关。

### 4.2 中立模型与 sing-box 字段的边界

| 类别 | 字段 | 放中立模型（结构化列） | 放 JSONB | 理由 |
|---|---|---|---|---|
| 跨协议通用 | `tag(name)` / `listen` / `port` / `server` / `server_port` | ✅ 结构化列 | — | 所有协议都要、且用于 tag 唯一/端口冲突校验 |
| 跨协议通用 | `tls` / `transport` | ✅ 结构化列 | — | 多协议复用，需共享渲染与组合校验 |
| 协议专属 | anytls `padding_scheme` / 会话池三件套 / `client_metadata` | ❌ | ✅ `settings` | 仅 anytls 有，且随版本演进（`client_metadata` 1.13.16 起） |
| 协议专属 | ss `method` / vmess `alterId` / tuic `congestion_control` 等 | ❌ | ✅ `settings` | 同上 |
| 敏感字段 | UUID / password / private_key | ❌（逻辑上） | ✅（物理上加密） | 🔐 AES-GCM 加密后仍存 JSONB 或专用列，解密只在渲染内存链路 |

**边界准则**：**「会参与跨协议通用校验（tag 唯一、端口、TLS 组合）」的字段才上升为结构化列；其余一律 JSONB。** 一旦某个协议专属字段开始被多个协议共享，再考虑「升列」。

### 4.3 协议组合合法性约束放哪一层

| 层 | 职责 | 是否权威 |
|---|---|---|
| **渲染器 `ValidateInbound/ValidateOutbound`（保存前）** | 协议级 + 协议×TLS/传输组合的**强校验**（如 anytls 禁 Reality、trojan 必选 TLS） | ✅ **唯一权威**（服务端，不可绕过） |
| **Generator（生成时）** | 跨协议全局校验：tag 全局唯一、引用完整性（routing 指向的 tag 必须存在）、端口冲突 | ✅ 权威（全局约束） |
| **前端（表单）** | 消费 `Meta()` 的允许清单做「体验层」约束（置灰/隐藏非法组合） | ❌ 非权威，仅提示 |

> 原则：**组合规则只写一份（在渲染器里），前端通过元数据反射得到约束**，避免规则在前端/后端/Generator 三处漂移。

---

## 5. 对既有设计（system_design.md）的影响

### 5.1 对 §3.1 目录结构

`configsvc/` 下新增 `renderer/` 子包（见 §2.1）。原 `template.go` 的「协议字段映射」职责**迁移**到 `renderer/`，`template.go` 只保留全局结构拼装。

### 5.2 对 §5 配置生成逻辑

- **§5.3 渲染模板**：vless 等协议的字段渲染从 `template.go` 抽到 `renderer/vless.go` 等；新增共享 `tls.go`/`transport.go`。
- **§5.4 字段映射表**：不再是一张集中大表，而是「通用字段（common.go）+ 各协议渲染器」的分布式映射。
- **§5.5 三层校验**：第一层「保存前业务校验」细化为「通用校验（validate.go）+ 协议校验（renderer.Validate）」。其余两层不变。

### 5.3 对 §7 任务列表 T02 的影响

T02（配置管理域）的**涉及文件**补充：

```
internal/configsvc/renderer/renderer.go、registry.go、tls.go、transport.go、common.go、
                vless.go、vmess.go、trojan.go、shadowsocks.go、hysteria2.go、tuic.go（首版 6 协议）、
                anytls.go（P1 案例）、*_test.go
internal/model/constants.go（协议类型白名单）
singbox-schema/inbound-*.json、outbound-*.json（按协议片段）
```

**建议在 T02 验收标准中新增一条**：

> 「新增一个协议」的验收路径：工程师新增 `renderer/foobar.go` + `init()` 注册 + Meta + 前端枚举 + schema 片段 + 单测后，**不改 Generator 主体、不改数据模型**，`Generate()` 即可产出合法 foobar JSON；以 anytls 作为该路径的验证用例。

### 5.4 是否需要迁移

| 变更 | 迁移类型 | 说明 |
|---|---|---|
| 新增 anytls 协议 | 无 DB 迁移 | `type varchar(32)` 无约束，靠代码白名单 |
| `outbound_config` 增加 `tls`/`transport` 列（§3.9 建议） | 1 条 `ALTER TABLE ... ADD COLUMN`（nullable） | 向后兼容，P0 建议纳入 T02 |

---

## 6. 待明确事项（建议与主理人/用户拍板）

1. **anytls 是否纳入 MVP 首版**：本设计按 P1 处理（作为「新增协议」演示用例），如需 P0，直接把 `anytls.go` + 前端枚举移入 T02 首版清单。
2. **`outbound_config` 是否补 `tls`/`transport` 列**：强烈建议补（§3.9），否则出站协议 TLS/transport 长期混在 `settings` 里。
3. **anytls 的 UDP 支持口径**：以 sing-box 1.12.x 官方文档为准（本文矩阵已标注 ⚠）；订阅 URI 细节以 mihomo/sing-box 解析实现为准。
4. **第二层 schema 校验策略**：是否每个协议维护独立 schema 片段（推荐），还是统一 `oneOf` 大 schema（维护重）。
5. **前端协议元数据来源**：推荐后端 `GET /protocols` 下发 `Meta()`，避免前后端字段定义漂移。
