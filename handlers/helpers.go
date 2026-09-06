package handlers

import (
	"errors"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
	"gorm.io/gorm"

	"librelock-server/middleware"
	"librelock-server/models"
)

// permitsShared reports whether the caller may perform a shared-vault write governed by the given
// organization setting. Admins and owners pass unconditionally; a missing organization row denies,
// which is the safe direction
func permitsShared(c *gin.Context, db *gorm.DB, allowed func(*models.Organization) bool) bool {
	user, ok := c.MustGet(middleware.UserKey).(*models.User)
	if !ok {
		return false
	}
	if models.IsAdminRole(user.Role) {
		return true
	}
	var org models.Organization
	if err := db.First(&org, "id = ?", models.OrgSingletonID).Error; err != nil {
		return false
	}
	return allowed(&org)
}

// The shared vault's two member permissions, from Organization -> Management -> Access
func canManageShared(o *models.Organization) bool { return o.MemberManageShared }

func canEditShared(o *models.Organization) bool { return o.MemberEditShared }

// Shared categories are both structure and entry content, so they take the two permissions together
func canCurateShared(o *models.Organization) bool {
	return o.MemberManageShared && o.MemberEditShared
}

func denyShared(c *gin.Context, action, subject string) {
	c.JSON(http.StatusForbidden, gin.H{"error": "Only admins can " + action + " shared " + subject})
}

// firstAccount returns the oldest account on the instance
// Personal mode has no roles, so it stands in for the operator: it alone may open sign-up or turn the instance into an organization
// Ties break on id so the answer never flips between calls
func firstAccount(db *gorm.DB) (models.User, error) {
	var first models.User
	err := db.Order("created_at asc, id asc").First(&first).Error
	return first, err
}

func isFirstAccount(db *gorm.DB, userID string) bool {
	first, err := firstAccount(db)
	return err == nil && first.ID == userID
}

// countActiveAdmins counts logged-in-able admins (owner included) for last-admin guards
// Every path that can remove an admin - demotion, suspension, removal, and self-deletion - has to consult the same count, or an organization can be left with nobody able to administer it
func countActiveAdmins(db *gorm.DB) int64 {
	var n int64
	db.Model(&models.User{}).
		Where("role IN ? AND status = ?",
			[]string{models.RoleAdmin, models.RoleOwner}, models.StatusActive).
		Count(&n)
	return n
}

func publicUser(u *models.User) map[string]any {
	return map[string]any{
		"id":              u.ID,
		"username":        u.Username,
		"role":            u.Role,
		"status":          u.Status,
		"theme":           u.Theme,
		"kdf_algo":        u.KDFAlgo,
		"kdf_salt":        u.KDFSalt,
		"kdf_iter":        u.KDFIter,
		"kdf_memory":      u.KDFMemory,
		"kdf_parallelism": u.KDFParallelism,
		"protected_key":   u.ProtectedKey,
		"public_key":      u.PublicKey,
		// encrypted_private_key is wrapped by the user's password key; only the account owner can decrypt it, so returning it to that user is safe
		"encrypted_private_key": u.EncryptedPrivateKey,
		"created_at":            u.CreatedAt,
		"updated_at":            u.UpdatedAt,
	}
}

func validationErrors(err error) map[string][]string {
	errs := make(map[string][]string)
	var ve validator.ValidationErrors
	if !errors.As(err, &ve) {
		errs["_"] = []string{err.Error()}
		return errs
	}
	for _, fe := range ve {
		field := fe.Field() // JSON tag name (registered in main.go)
		errs[field] = append(errs[field], fieldMessage(fe))
	}
	return errs
}

func fieldMessage(fe validator.FieldError) string {
	name := strings.ToLower(fe.Field())
	switch fe.Tag() {
	case "required":
		return "The " + name + " field is required."
	case "min":
		return "The " + name + " must be at least " + fe.Param() + "."
	case "max":
		return "The " + name + " may not be greater than " + fe.Param() + "."
	case "oneof":
		return "The selected " + name + " is invalid."
	case "uuid":
		return "The " + name + " must be a valid UUID."
	default:
		return "The " + name + " field is invalid."
	}
}
