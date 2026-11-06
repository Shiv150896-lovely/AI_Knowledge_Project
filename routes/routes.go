package routes

import (
	"net/http"

	"ai-knowledge-assistant/config"
	"ai-knowledge-assistant/handler"
	"ai-knowledge-assistant/middleware"
	"github.com/gin-gonic/gin"
)

func Register(r *gin.Engine, h handler.Handler, cfg config.Config) {
	r.Use(cors())
	r.GET("/health", func(c *gin.Context) { c.JSON(200, gin.H{"status": "ok"}) })
	r.GET("/assistant/:assistantToken", h.AssistantPage)
	a := r.Group("/api/auth")
	a.POST("/register", h.Register)
	a.POST("/login", h.Login)
	protected := r.Group("/api")
	protected.Use(middleware.Auth(cfg.JWTSecret))
	protected.POST("/auth/logout", h.Logout)
	protected.GET("/organization", h.Organization)
	protected.GET("/documents", h.Documents)
	protected.POST("/documents", h.Upload)
	protected.DELETE("/documents/:documentId", h.DeleteDocument)
	protected.GET("/assistant", h.Assistant)
	protected.POST("/assistant/regenerate-token", h.Regenerate)
	protected.GET("/usage", h.Usage)
	r.POST("/api/ask", h.AskQuestion)
}
func cors() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Assistant-Token")
		c.Header("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}
