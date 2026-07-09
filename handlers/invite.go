package handlers

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"librelock-server/crypto"
	"librelock-server/middleware"
	"librelock-server/models"
)

// defaultInviteDays is used when a request does not specify an expiry.
const defaultInviteDays = 1

func inviteStatus(i *models.Invite) string {
	switch {
	case i.IsUsed():
		return "used"
	case i.IsExpired():
		return "expired"
	default:
		return "pending"
	}
}

func inviteSummary(i *models.Invite) map[string]any {
	return map[string]any{
		"id":         i.ID,
		"note":       i.Note,
		"status":     inviteStatus(i),
		"created_by": i.CreatedBy,
		"expires_at": i.ExpiresAt,
		"used_at":    i.UsedAt,
		"created_at": i.CreatedAt,
	}
}

type createInviteRequest struct {
	Note          string `json:"note"            binding:"omitempty,max=200"`
	ExpiresInDays int    `json:"expires_in_days" binding:"omitempty,min=1,max=90"`
}

// CreateInvite mints a single-use invite token (admin only). The raw token is
// returned once — only its hash is stored.
func (h *OrganizationHandler) CreateInvite(c *gin.Context) {
	var req createInviteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}
	admin := c.MustGet(middleware.UserKey).(*models.User)

	days := req.ExpiresInDays
	if days == 0 {
		days = defaultInviteDays
	}

	token := crypto.IssueCode(16)
	invite := models.Invite{
		TokenHash: crypto.HashToken(token),
		Note:      req.Note,
		CreatedBy: admin.ID,
		ExpiresAt: time.Now().Add(time.Duration(days) * 24 * time.Hour),
	}
	if err := h.db.Create(&invite).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create invite"})
		return
	}
	recordAudit(h.db, AuditInviteCreated, admin, "", req.Note, fmt.Sprintf("expires in %d days", days))

	out := inviteSummary(&invite)
	out["token"] = token // shown once
	c.JSON(http.StatusCreated, gin.H{"invite": out})
}

// ListInvites returns invites newest first (admin only).
func (h *OrganizationHandler) ListInvites(c *gin.Context) {
	var invites []models.Invite
	if err := h.db.Order("created_at desc").Find(&invites).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load invites"})
		return
	}
	out := make([]map[string]any, 0, len(invites))
	for i := range invites {
		out = append(out, inviteSummary(&invites[i]))
	}
	c.JSON(http.StatusOK, gin.H{"invites": out})
}

// RevokeInvite deletes an invite (admin only).
func (h *OrganizationHandler) RevokeInvite(c *gin.Context) {
	var inv models.Invite
	if err := h.db.First(&inv, "id = ?", c.Param("id")).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Invite not found"})
		return
	}
	if err := h.db.Delete(&inv).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke invite"})
		return
	}
	actor := c.MustGet(middleware.UserKey).(*models.User)
	recordAudit(h.db, AuditInviteRevoked, actor, inv.ID, inv.Note, "")
	c.Status(http.StatusNoContent)
}
