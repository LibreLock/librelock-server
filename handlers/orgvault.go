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

type OrgVaultHandler struct{ db *gorm.DB }

func NewOrgVaultHandler(db *gorm.DB) *OrgVaultHandler { return &OrgVaultHandler{db: db} }

// orgCategoryExists reports whether an org category with the given id exists
func (h *OrgVaultHandler) orgCategoryExists(id string) bool {
	if _, err := uuid.Parse(id); err != nil {
		return false
	}
	var n int64
	h.db.Model(&models.OrgCategory{}).Where("id = ?", id).Count(&n)
	return n > 0
}

// Access is gated by middleware.RequireMembership, so every member with access sees every shared entry; there is no per-user scoping here by design

func (h *OrgVaultHandler) Index(c *gin.Context) {
	var entries []models.OrgVault
	h.db.Order("created_at asc").Find(&entries)
	c.JSON(http.StatusOK, gin.H{"entries": entries})
}

func (h *OrgVaultHandler) Show(c *gin.Context) {
	var entry models.OrgVault
	if err := h.db.First(&entry, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entry": entry})
}

type storeOrgVaultRequest struct {
	Type          string  `json:"type"           binding:"required,oneof=password_entry note card"`
	CategoryID    *string `json:"category_id"`
	EncryptedBlob string  `json:"encrypted_blob" binding:"required"`
	IV            string  `json:"iv"             binding:"required"`
	Version       *int    `json:"version"        binding:"omitempty,min=1"`
}

func (h *OrgVaultHandler) Store(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var req storeOrgVaultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}

	version := 1
	if req.Version != nil {
		version = *req.Version
	}

	if req.CategoryID != nil && !h.orgCategoryExists(*req.CategoryID) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid category_id"})
		return
	}

	creator := user.ID
	entry := models.OrgVault{
		CreatedBy:     &creator,
		CategoryID:    req.CategoryID,
		Type:          req.Type,
		EncryptedBlob: req.EncryptedBlob,
		IV:            req.IV,
		Version:       version,
	}
	if err := h.db.Create(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create entry"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"entry": entry})
}

func (h *OrgVaultHandler) Update(c *gin.Context) {
	var entry models.OrgVault
	if err := h.db.First(&entry, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Bad request"})
		return
	}

	var req struct {
		EncryptedBlob string  `json:"encrypted_blob"`
		IV            string  `json:"iv"`
		CategoryID    *string `json:"category_id"`
		Version       *int    `json:"version"`
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

	// Update the category only when the key is present in the body, so a plain edit does not clear it
	// A null value clears it; a non-null must exist
	var raw map[string]json.RawMessage
	json.Unmarshal(body, &raw)
	if _, hasCatID := raw["category_id"]; hasCatID {
		if req.CategoryID == nil {
			entry.CategoryID = nil
		} else {
			if !h.orgCategoryExists(*req.CategoryID) {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Invalid category_id"})
				return
			}
			entry.CategoryID = req.CategoryID
		}
	}

	entry.EncryptedBlob = req.EncryptedBlob
	entry.IV = req.IV
	if req.Version != nil {
		entry.Version = *req.Version
	}
	if err := h.db.Save(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update entry"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"entry": entry})
}

func (h *OrgVaultHandler) Destroy(c *gin.Context) {
	var entry models.OrgVault
	if err := h.db.First(&entry, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	h.db.Delete(&entry)
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}
