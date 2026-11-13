package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS 允许前端 dev 源与生产同源跨域（生产同源时浏览器不发送 Origin）。
func CORS() gin.HandlerFunc {
	allowed := map[string]bool{
		"http://localhost:5173": true, // vite 默认端口
		"http://localhost:3000": true, // 本仓库 vite 配置端口
		"http://127.0.0.1:5173": true,
		"http://127.0.0.1:3000": true,
	}
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" && allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
