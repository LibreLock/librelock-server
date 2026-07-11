package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User roles (organization mode only)
// Owner is the founder: a superset of admin, exactly one, and untouchable through user management
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

func IsAdminRole(role string) bool {
	return role == RoleAdmin || role == RoleOwner
}

// Suspended users keep their data but cannot log in
const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
)

type User struct {
	ID             string `gorm:"primaryKey;type:text"                           json:"id"`
	Username       string `gorm:"uniqueIndex;size:500;not null"                  json:"username"`
	Role           string `gorm:"size:20;not null;default:member"                json:"role"`
	Status         string `gorm:"size:20;not null;default:active"                json:"status"`
	Theme          string `gorm:"size:10;not null;default:dark"                  json:"theme"`
	AuthHash       string `gorm:"not null"                                       json:"-"`
	KDFAlgo        string `gorm:"column:kdf_algo;size:50;not null;default:argon2id" json:"kdf_algo"`
	KDFSalt        string `gorm:"column:kdf_salt;not null"                       json:"kdf_salt"`
	KDFIter        int    `gorm:"column:kdf_iter;not null"                       json:"kdf_iter"`
	KDFMemory      int    `gorm:"column:kdf_memory;not null"                     json:"kdf_memory"`
	KDFParallelism int    `gorm:"column:kdf_parallelism;not null"                json:"kdf_parallelism"`
	ProtectedKey   string `gorm:"column:protected_key;not null"                  json:"protected_key"`
	// Asymmetric keypair for organization sharing
	// PublicKey is stored plaintext; EncryptedPrivateKey is wrapped by the user's password-derived key
	PublicKey           string    `gorm:"column:public_key;not null;default:''"            json:"public_key"`
	EncryptedPrivateKey string    `gorm:"column:encrypted_private_key;not null;default:''" json:"encrypted_private_key"`
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

func (User) TableName() string { return "user" }

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	return nil
}
