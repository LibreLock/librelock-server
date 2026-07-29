package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"librelock-server/version"
)

// Version is public: the frontend shows it next to its own build so a stale tab or a half-finished upgrade is visible without logging in
func Version(c *gin.Context) {
	c.JSON(http.StatusOK, version.Get())
}
