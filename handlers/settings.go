package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"librelock-server/appmode"
	"librelock-server/config"
	"librelock-server/crypto"
	"librelock-server/middleware"
	"librelock-server/models"
)

type SettingsHandler struct {
	db   *gorm.DB
	env  string
	mode *appmode.Provider
}

func NewSettingsHandler(db *gorm.DB, env string, mode *appmode.Provider) *SettingsHandler {
	return &SettingsHandler{db: db, env: env, mode: mode}
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
			"username": {"Username taken"},
		}})
		return
	}

	if err := h.db.Model(user).UpdateColumn("username", newUsername).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": map[string][]string{
				"username": {"Username taken"},
			}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update username"})
		return
	}
	user.Username = newUsername
	c.JSON(http.StatusOK, gin.H{"user": publicUser(user)})
}

type updateThemeRequest struct {
	Theme string `json:"theme" binding:"required,oneof=light dark"`
}

func (h *SettingsHandler) UpdateTheme(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var req updateThemeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}
	if err := h.db.Model(user).UpdateColumn("theme", req.Theme).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update theme"})
		return
	}
	user.Theme = req.Theme
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

type switchModeRequest struct {
	Mode string `json:"mode" binding:"required,oneof=personal organization"`
	// Re-authenticates the owner for the destructive revert to personal.
	AuthCredential string `json:"auth_credential"`
}

// SwitchMode enables organization mode (caller becomes owner) or, given
// mode=personal, reverts to personal — see revertToPersonal.
func (h *SettingsHandler) SwitchMode(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)

	var req switchModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}

	if req.Mode == config.ModePersonal {
		h.revertToPersonal(c, user, req)
		return
	}

	// Switch to organization mode.
	if h.mode.IsOrganization() {
		c.JSON(http.StatusOK, gin.H{"mode": h.mode.Current()})
		return
	}
	if err := h.mode.EnableOrganization(user.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to switch to organization mode"})
		return
	}
	recordAudit(h.db, AuditModeChanged, user, user.ID, user.Username, "switched to organization")
	c.JSON(http.StatusOK, gin.H{"mode": h.mode.Current()})
}

// revertToPersonal is the destructive organization → personal downgrade:
// owner-only, password-confirmed, deletes every other account and their data.
func (h *SettingsHandler) revertToPersonal(c *gin.Context, user *models.User, req switchModeRequest) {
	if !h.mode.IsOrganization() {
		c.JSON(http.StatusOK, gin.H{"mode": h.mode.Current()})
		return
	}

	if user.Role != models.RoleOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the organization owner can do this"})
		return
	}

	if req.AuthCredential == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": map[string][]string{
			"auth_credential": {"Password confirmation is required."},
		}})
		return
	}
	ok, err := crypto.VerifyPassword(req.AuthCredential, user.AuthHash)
	if err != nil || !ok {
		c.JSON(http.StatusForbidden, gin.H{"error": "Invalid credentials"})
		return
	}

	// FK cascade removes each deleted user's vault, categories, and sessions.
	if err := h.db.Where("id != ?", user.ID).Delete(&models.User{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove other accounts"})
		return
	}

	h.db.Model(user).UpdateColumn("role", models.RoleMember)
	user.Role = models.RoleMember

	if err := h.mode.RevertToPersonal(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to switch to personal mode"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"mode": h.mode.Current()})
}
