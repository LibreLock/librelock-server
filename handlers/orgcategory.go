package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"librelock-server/models"
)

type OrgCategoryHandler struct{ db *gorm.DB }

func NewOrgCategoryHandler(db *gorm.DB) *OrgCategoryHandler {
	return &OrgCategoryHandler{db: db}
}

// Read paths are gated by middleware.RequireMembership; writes need both shared-vault permissions - a category is structure everyone sees and it files other members' entries (checked inline, since the routes still need membership for the org key)

func (h *OrgCategoryHandler) Index(c *gin.Context) {
	var categories []models.OrgCategory
	h.db.Order("created_at asc").Find(&categories)
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

func (h *OrgCategoryHandler) Show(c *gin.Context) {
	var category models.OrgCategory
	if err := h.db.First(&category, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"category": category})
}

type orgCategoryRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *OrgCategoryHandler) Store(c *gin.Context) {
	if !permitsShared(c, h.db, canCurateShared) {
		denyShared(c, "manage", "categories")
		return
	}
	var req orgCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}
	category := models.OrgCategory{Name: req.Name}
	if err := h.db.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"category": category})
}

func (h *OrgCategoryHandler) Update(c *gin.Context) {
	if !permitsShared(c, h.db, canCurateShared) {
		denyShared(c, "manage", "categories")
		return
	}
	var category models.OrgCategory
	if err := h.db.First(&category, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	var req orgCategoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}
	category.Name = req.Name
	h.db.Save(&category)
	c.JSON(http.StatusOK, gin.H{"category": category})
}

func (h *OrgCategoryHandler) Destroy(c *gin.Context) {
	if !permitsShared(c, h.db, canCurateShared) {
		denyShared(c, "manage", "categories")
		return
	}
	var category models.OrgCategory
	if err := h.db.First(&category, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	h.db.Delete(&category)
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}
