package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"

	"librelock-server/middleware"
	"librelock-server/models"
)

type VaultHandler struct{ db *gorm.DB }

func NewVaultHandler(db *gorm.DB) *VaultHandler { return &VaultHandler{db: db} }

func (h *VaultHandler) Index(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var entries []models.Vault
	h.db.Where("user_id = ?", user.ID).Order("created_at asc, id asc").Find(&entries)
	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

func (h *VaultHandler) Show(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var vault models.Vault
	if err := h.db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&vault).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entry": vault})
}

type storeVaultRequest struct {
	Type          string  `json:"type"           binding:"required,oneof=password_entry note card"`
	EncryptedBlob string  `json:"encrypted_blob" binding:"required"`
	IV            string  `json:"iv"             binding:"required"`
	CategoryID    *string `json:"category_id"`
	Version       *int    `json:"version"        binding:"omitempty,min=1"`
}

func (h *VaultHandler) Store(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var req storeVaultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}

	if req.CategoryID != nil {
		if _, err := uuid.Parse(*req.CategoryID); err != nil || !h.ownsCategory(user.ID, *req.CategoryID) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid category_id"})
			return
		}
	}

	version := 1
	if req.Version != nil {
		version = *req.Version
	}

	vault := models.Vault{
		UserID:        user.ID,
		CategoryID:    req.CategoryID,
		Type:          req.Type,
		EncryptedBlob: req.EncryptedBlob,
		IV:            req.IV,
		Version:       version,
	}
	if err := h.db.Create(&vault).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create entry"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"entry": vault})
}

func (h *VaultHandler) Update(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)

	var vault models.Vault
	if err := h.db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&vault).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bad request"})
		return
	}

	var req struct {
		EncryptedBlob string  `json:"encrypted_blob" binding:"required"`
		IV            string  `json:"iv"             binding:"required"`
		CategoryID    *string `json:"category_id"`
		Version       *int    `json:"version"        binding:"omitempty,min=1"`
	}
	if err := json.Unmarshal(body, &req); err != nil || req.EncryptedBlob == "" || req.IV == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": map[string][]string{
			"encrypted_blob": {"The encrypted_blob field is required."},
		}})
		return
	}
	if req.Version != nil && *req.Version < 1 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": map[string][]string{
			"version": {"The version must be at least 1."},
		}})
		return
	}

	// Detect if category_id was explicitly present in the JSON body
	var raw map[string]json.RawMessage
	json.Unmarshal(body, &raw)
	if _, hasCatID := raw["category_id"]; hasCatID {
		if req.CategoryID == nil {
			vault.CategoryID = nil
		} else {
			if _, err := uuid.Parse(*req.CategoryID); err != nil || !h.ownsCategory(user.ID, *req.CategoryID) {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid category_id"})
				return
			}
			vault.CategoryID = req.CategoryID
		}
	}

	vault.EncryptedBlob = req.EncryptedBlob
	vault.IV = req.IV
	if req.Version != nil {
		vault.Version = *req.Version
	}

	if err := h.db.Save(&vault).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update entry"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entry": vault})
}

func (h *VaultHandler) Destroy(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var vault models.Vault
	if err := h.db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&vault).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	h.db.Delete(&vault)
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}

func (h *VaultHandler) ownsCategory(userID, categoryID string) bool {
	var count int64
	h.db.Model(&models.Category{}).Where("id = ? AND user_id = ?", categoryID, userID).Count(&count)
	return count > 0
}
