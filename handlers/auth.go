package handlers

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"librelock-server/appmode"
	"librelock-server/crypto"
	"librelock-server/middleware"
	"librelock-server/models"
)

type AuthHandler struct {
	db     *gorm.DB
	ttl    int
	env    string
	mode   *appmode.Provider
	secret string // per-instance, keys the decoy KDF salts
}

func NewAuthHandler(db *gorm.DB, ttl int, env string, mode *appmode.Provider, secret string) *AuthHandler {
	return &AuthHandler{db: db, ttl: ttl, env: env, mode: mode, secret: secret}
}

// normalizeUsername must be applied identically everywhere a username is looked up, or the decoy KDF params for a miss won't line up with the account a later login resolves
func normalizeUsername(name string) string {
	return strings.TrimSpace(name)
}

// registrationPolicy reads the admin-set policy from the organization row, defaulting to invite-only if unset
// Only meaningful in organization mode
func (h *AuthHandler) registrationPolicy() string {
	var org models.Organization
	if err := h.db.First(&org, "id = ?", models.OrgSingletonID).Error; err != nil {
		return models.RegistrationInvite
	}
	if org.Registration == models.RegistrationOpen {
		return models.RegistrationOpen
	}
	return models.RegistrationInvite
}

func (h *AuthHandler) KDF(c *gin.Context) {
	username := normalizeUsername(c.Query("username"))
	var user models.User
	if err := h.db.Where("username = ?", username).First(&user).Error; err != nil {
		// Don't reveal the user doesn't exist: answer with the parameters this username would have been given at registration
		// The decoy salt matches a real one in shape and stays the same on every request
		c.JSON(http.StatusOK, gin.H{
			"kdf_algo":        "argon2id",
			"kdf_salt":        crypto.DecoyKDFSalt(h.secret, username),
			"kdf_iter":        minKDFIter,
			"kdf_memory":      minKDFMemory,
			"kdf_parallelism": defaultKDFParallelism,
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"kdf_algo":        user.KDFAlgo,
		"kdf_salt":        user.KDFSalt,
		"kdf_iter":        user.KDFIter,
		"kdf_memory":      user.KDFMemory,
		"kdf_parallelism": user.KDFParallelism,
	})
}

// Lower bounds for the client-chosen KDF parameters, mirroring KDF_ITER / KDF_MEMORY in src/constants.ts
// The client enforces the same floor on what the server returns, so neither side can talk the other into a weak derivation
// Struct tags can't reference consts, so the binding tags repeat these and carry the salt floor (min=64) alone
const (
	minKDFIter            = 4
	minKDFMemory          = 65536 // 64 MiB
	defaultKDFParallelism = 4
)

type registerRequest struct {
	Username       string              `json:"username"        binding:"required,max=500"`
	AuthCredential string              `json:"auth_credential" binding:"required,min=32,max=512"`
	ProtectedKey   string              `json:"protected_key"   binding:"required,min=32,max=1024"`
	KDFSalt        string              `json:"kdf_salt"        binding:"required,min=64,max=512"`
	KDFIter        int                 `json:"kdf_iter"        binding:"required,min=4,max=10000"`
	KDFMemory      int                 `json:"kdf_memory"      binding:"required,min=65536,max=1048576"`
	KDFParallelism int                 `json:"kdf_parallelism" binding:"required,min=1,max=16"`
	Categories     []categoryNameInput `json:"categories"`
	InviteToken    string              `json:"invite_token"`
	Theme          string              `json:"theme" binding:"omitempty,oneof=light dark"`
	// Sharing keypair
	// Optional for older clients; new clients always send both
	PublicKey           string `json:"public_key"            binding:"omitempty,max=4096"`
	EncryptedPrivateKey string `json:"encrypted_private_key" binding:"omitempty,max=8192"`
}

type categoryNameInput struct {
	Name string `json:"name" binding:"required"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}

	if len(req.Categories) > 20 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": map[string][]string{
			"categories": {"The categories may not have more than 20 items."},
		}})
		return
	}

	// Org mode: the first account becomes owner; afterwards invite mode requires a valid token
	// Personal mode leaves the role at its (unused) default
	// Running before the username check and before hashing means an invite-only instance gives a tokenless caller nothing: no "username taken" oracle and no 64 MiB of argon2 spent
	role := models.RoleMember
	var consumedInvite *models.Invite
	if h.mode.IsOrganization() {
		var userCount int64
		h.db.Model(&models.User{}).Count(&userCount)
		if userCount == 0 {
			role = models.RoleOwner // founder
		} else if h.registrationPolicy() == models.RegistrationInvite {
			inv, ok := h.findValidInvite(req.InviteToken)
			if !ok {
				c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": map[string][]string{
					"invite_token": {"This invite is invalid, expired, or already used."},
				}})
				return
			}
			consumedInvite = inv
		}
	}

	// Registration unavoidably reveals whether a name is taken, so open-registration instances are enumerable; /auth/kdf and login are not
	var count int64
	h.db.Model(&models.User{}).Where("username = ?", normalizeUsername(req.Username)).Count(&count)
	if count > 0 {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": map[string][]string{
			"username": {"Username taken"},
		}})
		return
	}

	authHash, err := crypto.HashPassword(req.AuthCredential)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	// Client sends the OS/browser-resolved theme at sign-up
	theme := req.Theme
	if theme == "" {
		theme = "dark"
	}

	user := models.User{
		Username:            normalizeUsername(req.Username),
		Role:                role,
		Theme:               theme,
		AuthHash:            authHash,
		KDFAlgo:             "argon2id",
		KDFSalt:             req.KDFSalt,
		KDFIter:             req.KDFIter,
		KDFMemory:           req.KDFMemory,
		KDFParallelism:      req.KDFParallelism,
		ProtectedKey:        req.ProtectedKey,
		PublicKey:           req.PublicKey,
		EncryptedPrivateKey: req.EncryptedPrivateKey,
	}
	if err := h.db.Create(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) {
			c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": map[string][]string{
				"username": {"Username taken"},
			}})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create user"})
		return
	}

	// Burn the invite now that the account exists
	if consumedInvite != nil {
		now := time.Now()
		h.db.Model(consumedInvite).Update("used_at", &now)
	}

	if h.mode.IsOrganization() {
		recordAudit(h.db, AuditUserRegistered, &user, user.ID, user.Username, "role: "+role)
	}

	for _, cat := range req.Categories {
		if cat.Name != "" {
			h.db.Create(&models.Category{UserID: user.ID, Name: cat.Name})
		}
	}

	token := crypto.IssueToken()
	if err := h.createSession(&user, token, c); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	h.setTokenCookie(c, token)
	c.JSON(http.StatusCreated, gin.H{"user": publicUser(&user)})
}

type loginRequest struct {
	Username       string `json:"username"        binding:"required"`
	AuthCredential string `json:"auth_credential" binding:"required"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"errors": validationErrors(err)})
		return
	}

	var user models.User
	if err := h.db.Where("username = ?", normalizeUsername(req.Username)).First(&user).Error; err != nil {
		// Spend the same argon2 work a real account would, so the response time doesn't say whether the username exists
		crypto.DummyVerify(req.AuthCredential)
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	ok, err := crypto.VerifyPassword(req.AuthCredential, user.AuthHash)
	if err != nil || !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	// Checked after password verification so suspension isn't a login oracle
	if user.Status == models.StatusSuspended {
		c.JSON(http.StatusForbidden, gin.H{"error": "Your account has been suspended."})
		return
	}

	token := crypto.IssueToken()
	if err := h.createSession(&user, token, c); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	h.setTokenCookie(c, token)
	c.JSON(http.StatusOK, gin.H{"user": publicUser(&user)})
}

func (h *AuthHandler) Logout(c *gin.Context) {
	if token, ok := c.Get(middleware.TokenKey); ok {
		h.db.Where("token_hash = ?", crypto.HashToken(token.(string))).Delete(&models.Session{})
	}
	h.clearTokenCookie(c)
	c.JSON(http.StatusOK, gin.H{"message": "Logged out"})
}

func (h *AuthHandler) Me(c *gin.Context) {
	user := c.MustGet(middleware.UserKey).(*models.User)
	c.JSON(http.StatusOK, gin.H{"user": publicUser(user)})
}

// findValidInvite returns the invite matching the raw token if it is unused and unexpired
func (h *AuthHandler) findValidInvite(token string) (*models.Invite, bool) {
	if token == "" {
		return nil, false
	}
	var inv models.Invite
	if err := h.db.Where("token_hash = ?", crypto.HashToken(token)).First(&inv).Error; err != nil {
		return nil, false
	}
	if !inv.IsValid() {
		return nil, false
	}
	return &inv, true
}

func (h *AuthHandler) createSession(user *models.User, token string, c *gin.Context) error {
	ua := c.GetHeader("User-Agent")
	if len(ua) > 255 {
		ua = ua[:255]
	}
	var deviceName *string
	if ua != "" {
		deviceName = &ua
	}
	session := models.Session{
		UserID:     user.ID,
		TokenHash:  crypto.HashToken(token),
		DeviceName: deviceName,
		IP:         c.ClientIP(),
		ExpiresAt:  time.Now().Add(time.Duration(h.ttl) * time.Second),
	}
	return h.db.Create(&session).Error
}

func (h *AuthHandler) setTokenCookie(c *gin.Context, token string) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", token, h.ttl, "/", "", h.env == "production", true)
}

func (h *AuthHandler) clearTokenCookie(c *gin.Context) {
	c.SetSameSite(http.SameSiteLaxMode)
	c.SetCookie("token", "", -1, "/", "", h.env == "production", true)
}
