package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"librelock-server/middleware"
	"librelock-server/models"
)

type SessionHandler struct{ db *gorm.DB }

func NewSessionHandler(db *gorm.DB) *SessionHandler { return &SessionHandler{db: db} }

func (h *SessionHandler) Index(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var sessions []models.Session
	// Expired rows are dead credentials, not sessions: the list must never show one, whether or not the purge has swept it yet
	h.db.Where("user_id = ? AND expires_at > ?", user.ID, time.Now()).
		Order("last_used_at DESC").Find(&sessions)
	c.JSON(http.StatusOK, gin.H{"sessions": sessions})
}

func (h *SessionHandler) Destroy(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var session models.Session
	if err := h.db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&session).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	h.db.Delete(&session)
	c.JSON(http.StatusOK, gin.H{"message": "Session destroyed"})
}

func (h *SessionHandler) DestroyAll(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	currentSessionID := c.MustGet(middleware.SessionIDKey).(string)
	h.db.Where("user_id = ? AND id != ?", user.ID, currentSessionID).Delete(&models.Session{})
	c.JSON(http.StatusOK, gin.H{"message": "All other sessions destroyed"})
}
