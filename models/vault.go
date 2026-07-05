package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Vault struct {
	ID            string    `gorm:"primaryKey;type:text"                           json:"id"`
	UserID        string    `gorm:"type:text;not null;index"                       json:"user_id"`
	User          User      `gorm:"constraint:OnDelete:CASCADE"                    json:"-"`
	CategoryID    *string   `gorm:"type:text;index"                                json:"category_id"`
	Category      *Category `gorm:"constraint:OnDelete:SET NULL"                   json:"-"`
	Type          string    `gorm:"size:20;not null;default:password_entry"        json:"type"`
	EncryptedBlob string    `gorm:"column:encrypted_blob;not null"                 json:"encrypted_blob"`
	IV            string    `gorm:"column:iv;not null"                             json:"iv"`
	Version       int       `gorm:"not null;default:1"                             json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Vault) TableName() string { return "vault" }

func (v *Vault) BeforeCreate(tx *gorm.DB) error {
	if v.ID == "" {
		v.ID = uuid.NewString()
	}
	return nil
}
