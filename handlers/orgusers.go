package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"librelock-server/middleware"
	"librelock-server/models"
)

func userSummary(u *models.User) map[string]any {
	return map[string]any{
		"id":         u.ID,
		"username":   u.Username,
		"role":       u.Role,
		"status":     u.Status,
		"created_at": u.CreatedAt,
		// Whether the user can receive shared access yet (has a sharing keypair)
		"has_public_key": u.PublicKey != "",
	}
}

// countActiveAdmins counts logged-in-able admins (owner included) for last-admin guards
func (h *OrganizationHandler) countActiveAdmins() int64 {
	var n int64
	h.db.Model(&models.User{}).
		Where("role IN ? AND status = ?",
			[]string{models.RoleAdmin, models.RoleOwner}, models.StatusActive).
		Count(&n)
	return n
}

// guardOwner blocks user-management actions against the (protected) owner account
func guardOwner(c *gin.Context, target *models.User) bool {
	if target.Role == models.RoleOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "The owner account cannot be modified"})
		return false
	}
	return true
}

// ListUsers returns every account (admin only)
func (h *OrganizationHandler) ListUsers(c *gin.Context) {
	var users []models.User
	if err := h.db.Order("created_at asc").Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load users"})
		return
	}
	out := make([]map[string]any, 0, len(users))
	for i := range users {
		out = append(out, userSummary(&users[i]))
	}
	c.JSON(http.StatusOK, gin.H{"users": out})
}

type updateRoleRequest struct {
	Role string `json:"role" binding:"required,oneof=owner admin member"`
}

// UpdateUserRole promotes/demotes a user; role "owner" is an ownership transfer
func (h *OrganizationHandler) UpdateUserRole(c *gin.Context) {
	var req updateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}

	target, ok := h.findUser(c)
	if !ok {
		return
	}
	if !guardOwner(c, target) {
		return
	}

	actor := c.MustGet(middleware.UserKey).(*models.User)

	if req.Role == models.RoleOwner {
		h.transferOwnership(c, actor, target)
		return
	}

	if target.Role == req.Role {
		c.JSON(http.StatusOK, gin.H{"user": userSummary(target)})
		return
	}
	if target.Role == models.RoleAdmin && req.Role == models.RoleMember && h.countActiveAdmins() <= 1 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Cannot demote the last admin"})
		return
	}

	prevRole := target.Role
	if err := h.db.Model(target).UpdateColumn("role", req.Role).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update role"})
		return
	}
	target.Role = req.Role
	recordAudit(h.db, AuditUserRoleChanged, actor, target.ID, target.Username, prevRole+" → "+req.Role)
	c.JSON(http.StatusOK, gin.H{"user": userSummary(target)})
}

// transferOwnership atomically makes target the owner and demotes the caller to admin
func (h *OrganizationHandler) transferOwnership(c *gin.Context, actor, target *models.User) {
	if actor.Role != models.RoleOwner {
		c.JSON(http.StatusForbidden, gin.H{"error": "Only the owner can transfer ownership"})
		return
	}
	if target.Status != models.StatusActive {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Cannot transfer ownership to a suspended user"})
		return
	}

	if err := h.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.User{}).Where("id = ?", target.ID).
			UpdateColumn("role", models.RoleOwner).Error; err != nil {
			return err
		}
		return tx.Model(&models.User{}).Where("id = ?", actor.ID).
			UpdateColumn("role", models.RoleAdmin).Error
	}); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to transfer ownership"})
		return
	}

	target.Role = models.RoleOwner
	recordAudit(h.db, AuditOwnershipTransferred, actor, target.ID, target.Username,
		actor.Username+" → admin")
	c.JSON(http.StatusOK, gin.H{"user": userSummary(target)})
}

type updateStatusRequest struct {
	Status string `json:"status" binding:"required,oneof=active suspended"`
}

// UpdateUserStatus suspends (keeps the vault, kills sessions) or reactivates a user
func (h *OrganizationHandler) UpdateUserStatus(c *gin.Context) {
	var req updateStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}

	target, ok := h.findUser(c)
	if !ok {
		return
	}
	if !guardOwner(c, target) {
		return
	}

	if target.Status == req.Status {
		c.JSON(http.StatusOK, gin.H{"user": userSummary(target)})
		return
	}
	if req.Status == models.StatusSuspended && target.Role == models.RoleAdmin && h.countActiveAdmins() <= 1 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Cannot suspend the last admin"})
		return
	}

	if err := h.db.Model(target).UpdateColumn("status", req.Status).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update status"})
		return
	}
	target.Status = req.Status
	actor := c.MustGet(middleware.UserKey).(*models.User)

	if req.Status == models.StatusSuspended {
		// Force logout everywhere so suspension is immediate, not next-login
		h.db.Where("user_id = ?", target.ID).Delete(&models.Session{})
		recordAudit(h.db, AuditUserSuspended, actor, target.ID, target.Username, "")
	} else {
		recordAudit(h.db, AuditUserReactivated, actor, target.ID, target.Username, "")
	}
	c.JSON(http.StatusOK, gin.H{"user": userSummary(target)})
}

// RemoveUser deletes a user and cascade-deletes all their data
func (h *OrganizationHandler) RemoveUser(c *gin.Context) {
	target, ok := h.findUser(c)
	if !ok {
		return
	}
	if !guardOwner(c, target) {
		return
	}

	if target.Role == models.RoleAdmin && h.countActiveAdmins() <= 1 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "Cannot remove the last admin"})
		return
	}

	id, name := target.ID, target.Username
	if err := h.db.Delete(target).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove user"})
		return
	}
	actor := c.MustGet(middleware.UserKey).(*models.User)
	recordAudit(h.db, AuditUserRemoved, actor, id, name, "")
	c.Status(http.StatusNoContent)
}

// findUser loads the :id user, writing the error response if not found
func (h *OrganizationHandler) findUser(c *gin.Context) (*models.User, bool) {
	current := c.MustGet(middleware.UserKey).(*models.User)
	id := c.Param("id")
	if id == current.ID {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "You cannot modify your own account here"})
		return nil, false
	}
	var user models.User
	if err := h.db.First(&user, "id = ?", id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return nil, false
	}
	return &user, true
}
