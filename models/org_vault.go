package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OrgVault is a shared entry encrypted with the organization key
// Deliberately has no owning-user foreign key: shared entries outlive the member who created them, and access is gated by OrgVaultMembership, not row ownership
type OrgVault struct {
	ID            string       `gorm:"primaryKey;type:text"                    json:"id"`
	CreatedBy     *string      `gorm:"type:text"                               json:"created_by"`
	CategoryID    *string      `gorm:"type:text;index"                         json:"category_id"`
	Category      *OrgCategory `gorm:"constraint:OnDelete:SET NULL"            json:"-"`
	Type          string       `gorm:"size:20;not null;default:password_entry" json:"type"`
	EncryptedBlob string       `gorm:"column:encrypted_blob;not null"          json:"encrypted_blob"`
	IV            string       `gorm:"column:iv;not null"                      json:"iv"`
	Version       int          `gorm:"not null;default:1"                      json:"version"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

func (OrgVault) TableName() string { return "org_vault" }

func (v *OrgVault) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = uuid.NewString()
	}
	return nil
}
