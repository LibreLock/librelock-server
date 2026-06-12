package handlers

import (
	"errors"
	"strings"

	"github.com/go-playground/validator/v10"

	"librelock-server/models"
)

func publicUser(u *models.User) map[string]any {
	return map[string]any{
		"id":              u.ID,
		"username":        u.Username,
		"kdf_algo":        u.KDFAlgo,
		"kdf_salt":        u.KDFSalt,
		"kdf_iter":        u.KDFIter,
		"kdf_memory":      u.KDFMemory,
		"kdf_parallelism": u.KDFParallelism,
		"protected_key":   u.ProtectedKey,
		"created_at":      u.CreatedAt,
		"updated_at":      u.UpdatedAt,
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
