package handlers

import (
	"encoding/base64"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"librelock-server/appmode"
	"librelock-server/config"
	"librelock-server/middleware"
	"librelock-server/models"
)

const maxLogoBytes = 512 * 1024 // 512 KiB

var allowedLogoMimes = map[string]bool{
	"image/png":     true,
	"image/jpeg":    true,
	"image/svg+xml": true,
	"image/webp":    true,
	"image/gif":     true,
}

type OrganizationHandler struct {
	db   *gorm.DB
	mode *appmode.Provider
}

func NewOrganizationHandler(db *gorm.DB, mode *appmode.Provider) *OrganizationHandler {
	return &OrganizationHandler{db: db, mode: mode}
}

// getOrCreate returns the singleton org row, creating a default one if absent.
func (h *OrganizationHandler) getOrCreate() (*models.Organization, error) {
	var org models.Organization
	err := h.db.First(&org, "id = ?", models.OrgSingletonID).Error
	if err == nil {
		return &org, nil
	}
	if err != gorm.ErrRecordNotFound {
		return nil, err
	}
	org = models.Organization{
		ID:           models.OrgSingletonID,
		Name:         "LibreLock",
		Registration: models.RegistrationInvite, // safer default
	}
	if err := h.db.Create(&org).Error; err != nil {
		return nil, err
	}
	return &org, nil
}

func publicOrg(o *models.Organization) map[string]any {
	return map[string]any{
		"name":            o.Name,
		"support_email":   o.SupportEmail,
		"support_url":     o.SupportURL,
		"login_message":   o.LoginMessage,
		"has_logo":        o.HasLogo(),
		"logo_updated_at": o.UpdatedAt,
	}
}

// orgPayload is publicOrg plus mode/registration the frontend needs on every response.
func (h *OrganizationHandler) orgPayload(o *models.Organization) map[string]any {
	p := publicOrg(o)
	p["mode"] = h.mode.Current()
	p["registration"] = registrationOrDefault(o.Registration)
	return p
}

// personalPayload is the branding response for a personal instance: plain
// LibreLock, no org row touched (the organization table may not even exist).
func personalPayload() map[string]any {
	return map[string]any{
		"name":            "LibreLock",
		"support_email":   "",
		"support_url":     "",
		"login_message":   "",
		"has_logo":        false,
		"logo_updated_at": time.Time{},
		"mode":            config.ModePersonal,
		"registration":    models.RegistrationOpen,
	}
}

// registrationOrDefault normalises a possibly-empty stored value.
func registrationOrDefault(v string) string {
	if v == models.RegistrationOpen {
		return models.RegistrationOpen
	}
	return models.RegistrationInvite
}

// Show is public — the frontend calls it on load to swap branding.
func (h *OrganizationHandler) Show(c *gin.Context) {
	if !h.mode.IsOrganization() {
		c.JSON(http.StatusOK, gin.H{"organization": personalPayload()})
		return
	}
	org, err := h.getOrCreate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load organization"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"organization": h.orgPayload(org)})
}

// Logo is public — serves the raw logo bytes for <img> tags.
func (h *OrganizationHandler) Logo(c *gin.Context) {
	if !h.mode.IsOrganization() {
		c.Status(http.StatusNotFound)
		return
	}
	org, err := h.getOrCreate()
	if err != nil || !org.HasLogo() {
		c.Status(http.StatusNotFound)
		return
	}
	// Public branding asset embedded by the web app on a different origin,
	// so relax the global same-origin resource policy for this response.
	c.Header("Cross-Origin-Resource-Policy", "cross-origin")
	c.Header("Cache-Control", "no-cache")
	c.Data(http.StatusOK, org.LogoMimeType, org.LogoData)
}

type updateOrgRequest struct {
	Name         *string `json:"name"          binding:"omitempty,max=200"`
	SupportEmail *string `json:"support_email" binding:"omitempty,max=200"`
	SupportURL   *string `json:"support_url"   binding:"omitempty,max=300"`
	LoginMessage *string `json:"login_message" binding:"omitempty,max=500"`
}

// Update edits branding fields (auth required).
func (h *OrganizationHandler) Update(c *gin.Context) {
	var req updateOrgRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}
	org, err := h.getOrCreate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load organization"})
		return
	}

	var changed []string
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			name = "LibreLock"
		}
		org.Name = name
		changed = append(changed, "name")
	}
	if req.SupportEmail != nil {
		org.SupportEmail = strings.TrimSpace(*req.SupportEmail)
		changed = append(changed, "support email")
	}
	if req.SupportURL != nil {
		org.SupportURL = strings.TrimSpace(*req.SupportURL)
		changed = append(changed, "support URL")
	}
	if req.LoginMessage != nil {
		org.LoginMessage = strings.TrimSpace(*req.LoginMessage)
		changed = append(changed, "login message")
	}

	if err := h.db.Save(org).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update organization"})
		return
	}
	actor := c.MustGet(middleware.UserKey).(*models.User)
	recordAudit(h.db, AuditOrgUpdated, actor, "", "", strings.Join(changed, ", "))
	c.JSON(http.StatusOK, gin.H{"organization": h.orgPayload(org)})
}

type uploadLogoRequest struct {
	// Data is a data URL, e.g. "data:image/png;base64,iVBORw0KG..."
	Data string `json:"data" binding:"required"`
}

// UploadLogo stores a new logo from a base64 data URL (auth required).
func (h *OrganizationHandler) UploadLogo(c *gin.Context) {
	var req uploadLogoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}

	mime, raw, ok := parseDataURL(req.Data)
	if !ok || !allowedLogoMimes[mime] {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": map[string][]string{
			"data": {"Logo must be a PNG, JPEG, SVG, WebP or GIF data URL."},
		}})
		return
	}
	if len(raw) > maxLogoBytes {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": map[string][]string{
			"data": {"Logo must be 512 KiB or smaller."},
		}})
		return
	}

	org, err := h.getOrCreate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load organization"})
		return
	}
	org.LogoData = raw
	org.LogoMimeType = mime
	org.UpdatedAt = time.Now()
	if err := h.db.Save(org).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save logo"})
		return
	}
	actor := c.MustGet(middleware.UserKey).(*models.User)
	recordAudit(h.db, AuditOrgLogoUpdated, actor, "", "", "")
	c.JSON(http.StatusOK, gin.H{"organization": h.orgPayload(org)})
}

// DeleteLogo resets branding back to the default LibreLock padlock.
func (h *OrganizationHandler) DeleteLogo(c *gin.Context) {
	org, err := h.getOrCreate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load organization"})
		return
	}
	if err := h.db.Model(org).Updates(map[string]any{
		"logo_data":      nil,
		"logo_mime_type": "",
	}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove logo"})
		return
	}
	org.LogoData = nil
	org.LogoMimeType = ""
	actor := c.MustGet(middleware.UserKey).(*models.User)
	recordAudit(h.db, AuditOrgLogoRemoved, actor, "", "", "")
	c.JSON(http.StatusOK, gin.H{"organization": h.orgPayload(org)})
}

type updateRegistrationRequest struct {
	Registration string `json:"registration" binding:"required,oneof=open invite"`
}

// UpdateRegistration toggles invite-only vs open sign-up (audited).
func (h *OrganizationHandler) UpdateRegistration(c *gin.Context) {
	var req updateRegistrationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}
	org, err := h.getOrCreate()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to load organization"})
		return
	}
	if org.Registration == req.Registration {
		c.JSON(http.StatusOK, gin.H{"organization": h.orgPayload(org)})
		return
	}
	if err := h.db.Model(org).UpdateColumn("registration", req.Registration).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update registration"})
		return
	}
	org.Registration = req.Registration
	actor := c.MustGet(middleware.UserKey).(*models.User)
	recordAudit(h.db, AuditRegistrationChanged, actor, "", "", req.Registration)
	c.JSON(http.StatusOK, gin.H{"organization": h.orgPayload(org)})
}

// parseDataURL splits "data:<mime>;base64,<payload>" into mime + decoded bytes.
func parseDataURL(s string) (mime string, data []byte, ok bool) {
	if !strings.HasPrefix(s, "data:") {
		return "", nil, false
	}
	rest := strings.TrimPrefix(s, "data:")
	comma := strings.IndexByte(rest, ',')
	if comma < 0 {
		return "", nil, false
	}
	meta, payload := rest[:comma], rest[comma+1:]
	if !strings.Contains(meta, ";base64") {
		return "", nil, false
	}
	mime = strings.TrimSpace(strings.SplitN(meta, ";", 2)[0])
	raw, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return "", nil, false
	}
	return mime, raw, true
}
