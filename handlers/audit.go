package handlers

import (
	"log"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"librelock-server/models"
)

// Audit action identifiers
const (
	AuditUserRegistered        = "user.registered"
	AuditUserRoleChanged       = "user.role_changed"
	AuditOwnershipTransferred  = "user.ownership_transferred"
	AuditUserSuspended         = "user.suspended"
	AuditUserReactivated       = "user.reactivated"
	AuditUserRemoved           = "user.removed"
	AuditInviteCreated         = "invite.created"
	AuditInviteRevoked         = "invite.revoked"
	AuditOrgUpdated            = "org.updated"
	AuditOrgLogoUpdated        = "org.logo_updated"
	AuditOrgLogoRemoved        = "org.logo_removed"
	AuditRegistrationChanged   = "org.registration_changed"
	AuditModeChanged           = "app.mode_changed"
	AuditSharedAccessGranted   = "org.shared_access_granted"
	AuditSharedAccessRevoked   = "org.shared_access_revoked"
	AuditSharedSettingsChanged = "org.shared_settings_changed"
)

// recordAudit writes a best-effort audit entry; a failure is logged, never surfaced
func recordAudit(db *gorm.DB, action string, actor *models.User, targetID, targetName, detail string) {
	ev := models.AuditEvent{
		Action:     action,
		TargetID:   targetID,
		TargetName: targetName,
		Detail:     detail,
	}
	if actor != nil {
		ev.ActorID = actor.ID
		ev.ActorName = actor.Username
	}
	if err := db.Create(&ev).Error; err != nil {
		log.Printf("audit: failed to record %s: %v", action, err)
	}
}

func auditSummary(a *models.AuditEvent) map[string]any {
	return map[string]any{
		"id":          a.ID,
		"action":      a.Action,
		"actor_id":    a.ActorID,
		"actor_name":  a.ActorName,
		"target_id":   a.TargetID,
		"target_name": a.TargetName,
		"detail":      a.Detail,
		"created_at":  a.CreatedAt,
	}
}

// ListAuditEvents returns recent audit entries newest first (admin only)
func (h *OrganizationHandler) ListAuditEvents(c *gin.Context) {
	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	var events []models.AuditEvent
	if err := h.db.Order("created_at desc").Limit(limit).Find(&events).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load audit log"})
		return
	}
	out := make([]map[string]any, 0, len(events))
	for i := range events {
		out = append(out, auditSummary(&events[i]))
	}
	c.JSON(http.StatusOK, gin.H{"events": out})
}
