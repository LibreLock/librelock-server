package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm/clause"

	"librelock-server/middleware"
	"librelock-server/models"
)

// MyOrgKey returns the caller's wrapped shared-vault key, if they have access
// The client unwraps it with its private key to read shared entries
func (h *OrganizationHandler) MyOrgKey(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	var m models.OrgVaultMembership
	if err := h.db.First(&m, "user_id = ?", user.ID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "No shared access"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"wrapped_key": m.WrappedKey})
}

// ListMemberships returns every account with its public key and whether it currently has shared access: everything the grant UI needs in one call
func (h *OrganizationHandler) ListMemberships(c *gin.Context) {
	var users []models.User
	if err := h.db.Order("created_at asc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load users"})
		return
	}

	var memberships []models.OrgVaultMembership
	h.db.Find(&memberships)
	granted := make(map[string]bool, len(memberships))
	for _, m := range memberships {
		granted[m.UserID] = true
	}

	out := make([]map[string]any, 0, len(users))
	for i := range users {
		u := &users[i]
		out = append(out, map[string]any{
			"user_id":    u.ID,
			"username":   u.Username,
			"role":       u.Role,
			"status":     u.Status,
			"public_key": u.PublicKey,
			"has_access": granted[u.ID],
		})
	}
	c.JSON(http.StatusOK, gin.H{"members": out})
}

type grantMembershipRequest struct {
	UserID     string `json:"user_id"     binding:"required"`
	WrappedKey string `json:"wrapped_key" binding:"required,max=4096"`
}

// GrantMembership stores the shared org key enveloped to a member's public key
// Upserts so re-granting and the owner's bootstrap self-grant are idempotent
func (h *OrganizationHandler) GrantMembership(c *gin.Context) {
	var req grantMembershipRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}

	var target models.User
	if err := h.db.First(&target, "id = ?", req.UserID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if target.Status != models.StatusActive {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Cannot grant access to a suspended user"})
		return
	}
	if target.PublicKey == "" {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "This user has not set up a sharing key yet"})
		return
	}

	actor := c.MustGet(middleware.UserKey).(*models.User)
	m := models.OrgVaultMembership{
		UserID:     target.ID,
		WrappedKey: req.WrappedKey,
		GrantedBy:  actor.ID,
	}
	if err := h.db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "user_id"}},
		DoUpdates: clause.AssignmentColumns([]string{"wrapped_key", "granted_by", "updated_at"}),
	}).Create(&m).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to grant access"})
		return
	}

	recordAudit(h.db, AuditSharedAccessGranted, actor, target.ID, target.Username, "")
	c.JSON(http.StatusOK, gin.H{"membership": gin.H{"user_id": target.ID}})
}

// RevokeMembership removes a member's shared access
// Note: the member already saw the org key, so true secrecy requires rotating the key (a later feature)
func (h *OrganizationHandler) RevokeMembership(c *gin.Context) {
	id := c.Param("userId")

	var target models.User
	if err := h.db.First(&target, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}
	if target.Role == models.RoleOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "The owner cannot lose shared access"})
		return
	}

	if err := h.db.Where("user_id = ?", id).Delete(&models.OrgVaultMembership{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke access"})
		return
	}

	actor := c.MustGet(middleware.UserKey).(*models.User)
	recordAudit(h.db, AuditSharedAccessRevoked, actor, target.ID, target.Username, "")
	c.Status(http.StatusNoContent)
}
