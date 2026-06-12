package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"librelock-server/crypto"
	"librelock-server/models"
)

const (
	UserKey      = "user"
	SessionIDKey = "session_id"
	TokenKey     = "token"
)

func Auth(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := extractToken(c)
		if token == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		var session models.Session
		if err := db.Where("token_hash = ? AND expires_at > ?", crypto.HashToken(token), time.Now()).
			First(&session).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Session expired or invalid"})
			return
		}

		db.Model(&session).UpdateColumn("last_used_at", time.Now())

		var user models.User
		if err := db.First(&user, "id = ?", session.UserID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		c.Set(UserKey, &user)
		c.Set(SessionIDKey, session.ID)
		c.Set(TokenKey, token)
		c.Next()
	}
}

func extractToken(c *gin.Context) string {
	if token, err := c.Cookie("token"); err == nil && token != "" {
		return token
	}
	if auth := c.GetHeader("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		return strings.TrimPrefix(auth, "Bearer ")
	}
	return ""
}
