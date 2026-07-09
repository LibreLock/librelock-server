package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// User roles (organization mode only). Owner is the founder: a superset of
// admin, exactly one, and untouchable through user management.
const (
	RoleOwner  = "owner"
	RoleAdmin  = "admin"
	RoleMember = "member"
)

func IsAdminRole(role string) bool {
	return role == RoleAdmin || role == RoleOwner
}

// Suspended users keep their data but cannot log in.
const (
	StatusActive    = "active"
	StatusSuspended = "suspended"
)

type User struct {
	ID             string    `gorm:"primaryKey;type:text"                           json:"id"`
	Username       string    `gorm:"uniqueIndex;size:200;not null"                  json:"username"`
	Role           string    `gorm:"size:20;not null;default:member"                json:"role"`
	Status         string    `gorm:"size:20;not null;default:active"                json:"status"`
	Theme          string    `gorm:"size:10;not null;default:dark"                  json:"theme"`
	AuthHash       string    `gorm:"not null"                                       json:"-"`
	KDFAlgo        string    `gorm:"column:kdf_algo;size:50;not null;default:argon2id" json:"kdf_algo"`
	KDFSalt        string    `gorm:"column:kdf_salt;not null"                       json:"kdf_salt"`
	KDFIter        int       `gorm:"column:kdf_iter;not null"                       json:"kdf_iter"`
	KDFMemory      int       `gorm:"column:kdf_memory;not null"                     json:"kdf_memory"`
	KDFParallelism int       `gorm:"column:kdf_parallelism;not null"                json:"kdf_parallelism"`
	ProtectedKey   string    `gorm:"column:protected_key;not null"                  json:"protected_key"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (User) TableName() string { return "user" }

func (u *User) BeforeCreate(tx *gorm.DB) error {
	if u.ID == "" {
		u.ID = uuid.NewString()
	}
	return nil
}
