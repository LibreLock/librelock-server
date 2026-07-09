package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"librelock-server/appmode"
	"librelock-server/models"
)

// RequireAdmin permits admins (owner included) in organization mode only.
// Reads the mode live and must run after Auth.
func RequireAdmin(mode *appmode.Provider) gin.HandlerFunc {
	return func(c *gin.Context) {
		if !mode.IsOrganization() {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Not available in personal mode",
			})
			return
		}
		user, ok := c.MustGet(UserKey).(*models.User)
		if !ok || !models.IsAdminRole(user.Role) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error": "Admin access required",
			})
			return
		}
		c.Next()
	}
}
