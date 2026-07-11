package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"librelock-server/appmode"
	"librelock-server/models"
)

// RequireMembership permits only users with shared-vault access in organization mode
// Reads the mode live and must run after Auth
func RequireMembership(db *gorm.DB, mode *appmode.Provider) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !mode.IsOrganization() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Not available in personal mode",
			})
			return
		}
		user, ok := c.MustGet(UserKey).(*models.User)
		if !ok {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Shared access required"})
			return
		}
		var n int64
		db.Model(&models.OrgVaultMembership{}).Where("user_id = ?", user.ID).Count(&n)
		if n == 0 {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Shared access required"})
			return
		}
		c.Next()
	}
}
