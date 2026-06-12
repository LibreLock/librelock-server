package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"librelock-server/middleware"
	"librelock-server/models"
)

type CategoryHandler struct{ db *gorm.DB }

func NewCategoryHandler(db *gorm.DB) *CategoryHandler { return &CategoryHandler{db: db} }

func (h *CategoryHandler) Index(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var categories []models.Category
	h.db.Where("user_id = ?", user.ID).Find(&categories)
	c.JSON(http.StatusOK, gin.H{"categories": categories})
}

func (h *CategoryHandler) Show(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var category models.Category
	if err := h.db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&category).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"category": category})
}

type categoryRequest struct {
	Name string `json:"name" binding:"required"`
}

func (h *CategoryHandler) Store(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}
	category := models.Category{UserID: user.ID, Name: req.Name}
	if err := h.db.Create(&category).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create category"})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"category": category})
}

func (h *CategoryHandler) Update(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var category models.Category
	if err := h.db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&category).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	var req categoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}
	category.Name = req.Name
	h.db.Save(&category)
	c.JSON(http.StatusOK, gin.H{"category": category})
}

func (h *CategoryHandler) Destroy(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var category models.Category
	if err := h.db.Where("id = ? AND user_id = ?", c.Param("id"), user.ID).First(&category).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Not found"})
		return
	}
	h.db.Delete(&category)
	c.JSON(http.StatusOK, gin.H{"message": "Deleted"})
}
