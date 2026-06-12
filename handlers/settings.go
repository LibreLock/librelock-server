package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"librelock-server/crypto"
	"librelock-server/middleware"
	"librelock-server/models"
)

type SettingsHandler struct {
	db  *gorm.DB
	env string
}

func NewSettingsHandler(db *gorm.DB, env string) *SettingsHandler {
	return &SettingsHandler{db: db, env: env}
}

type updateUsernameRequest struct {
	Username string `json:"username" binding:"required,max=200"`
}

func (h *SettingsHandler) UpdateUsername(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var req updateUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}

	newUsername := strings.TrimSpace(req.Username)
	var count int64
	h.db.Model(&models.User{}).Where("username = ?", newUsername).Count(&count)
	if count > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": map[string][]string{
			"username": {"The username has already been taken."},
		}})
		return
	}

	if err := h.db.Model(user).UpdateColumn("username", newUsername).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update username"})
		return
	}
	user.Username = newUsername
	c.JSON(http.StatusOK, gin.H{"user": publicUser(user)})
}

type updatePasswordRequest struct {
	CurrentAuthCredential string `json:"current_auth_credential" binding:"required"`
	NewAuthCredential     string `json:"new_auth_credential"     binding:"required,min=32,max=512"`
	NewProtectedKey       string `json:"new_protected_key"       binding:"required,min=32,max=1024"`
	NewKDFSalt            string `json:"new_kdf_salt"            binding:"required,min=16,max=512"`
	NewKDFIter            int    `json:"new_kdf_iter"            binding:"required,min=1,max=10000"`
	NewKDFMemory          int    `json:"new_kdf_memory"          binding:"required,min=8192,max=1048576"`
	NewKDFParallelism     int    `json:"new_kdf_parallelism"     binding:"required,min=1,max=16"`
}

func (h *SettingsHandler) UpdateMasterPassword(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var req updatePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}

	ok, err := crypto.VerifyPassword(req.CurrentAuthCredential, user.AuthHash)
	if err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid credentials"})
		return
	}

	newHash, err := crypto.HashPassword(req.NewAuthCredential)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	currentTokenHash := crypto.HashToken(c.MustGet(middleware.TokenKey).(string))

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(user).Updates(map[string]any{
			"auth_hash":       newHash,
			"protected_key":   req.NewProtectedKey,
			"kdf_salt":        req.NewKDFSalt,
			"kdf_iter":        req.NewKDFIter,
			"kdf_memory":      req.NewKDFMemory,
			"kdf_parallelism": req.NewKDFParallelism,
		}).Error; err != nil {
			return err
		}
		// Invalidate all other sessions
		return tx.Where("user_id = ? AND token_hash != ?", user.ID, currentTokenHash).
			Delete(&models.Session{}).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update password"})
		return
	}

	// Refresh user from DB to get updated timestamps
	h.db.First(user, "id = ?", user.ID)
	c.JSON(http.StatusOK, gin.H{"user": publicUser(user)})
}

type deleteAccountRequest struct {
	AuthCredential string `json:"auth_credential" binding:"required"`
}

func (h *SettingsHandler) DeleteAccount(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var req deleteAccountRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}

	ok, err := crypto.VerifyPassword(req.AuthCredential, user.AuthHash)
	if err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid credentials"})
		return
	}

	// Cascade handled by DB foreign keys
	if err := h.db.Delete(user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete account"})
		return
	}

	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", "", -1, "/", "", h.env == "production", true)
	c.JSON(http.StatusOK, gin.H{"message": "Account deleted"})
}
