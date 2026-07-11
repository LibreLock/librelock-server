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

func Auth(db *gorm.DB, ttl int, env string) gin.HandlerFunc {
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

		now := time.Now()
		updates := map[string]any{"last_used_at": now}
		// Sliding expiration: an active user should never be logged out mid-session Once past the halfway point of the window, push the expiry forward and refresh the cookie's max-age to match
		if ttl > 0 && time.Until(session.ExpiresAt) < time.Duration(ttl)*time.Second/2 {
			updates["expires_at"] = now.Add(time.Duration(ttl) * time.Second)
			setTokenCookie(c, token, ttl, env)
		}
		db.Model(&session).Updates(updates)

		var user models.User
		if err := db.First(&user, "id = ?", session.UserID).Error; err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "User not found"})
			return
		}

		// A suspended user's live sessions must stop working immediately
		if user.Status == models.StatusSuspended {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Account suspended"})
			return
		}

		c.Set(UserKey, &user)
		c.Set(SessionIDKey, session.ID)
		c.Set(TokenKey, token)
		c.Next()
	}
}

// setTokenCookie mirrors the handler's cookie settings so a renewed session's cookie stays consistent with the one issued at login
func setTokenCookie(c *gin.Context, token string, ttl int, env string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", token, ttl, "/", "", env == "production", true)
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
