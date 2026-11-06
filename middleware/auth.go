package middleware

import (
	"net/http"
	"strings"

	"ai-knowledge-assistant/Utils"
	"github.com/gin-gonic/gin"
)

func Auth(secret string) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": "missing bearer token"}})
			return
		}
		uid, org, err := utils.ParseJWT(strings.TrimPrefix(h, "Bearer "), secret)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": gin.H{"code": "unauthorized", "message": "invalid bearer token"}})
			return
		}
		c.Set("userID", uid)
		c.Set("organizationID", org)
		c.Next()
	}
}
func OrganizationID(c *gin.Context) string { return c.MustGet("organizationID").(string) }
