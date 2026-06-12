package models

import "time"

type Vault struct {
	ID            string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID        string    `gorm:"type:uuid;not null;index"                       json:"user_id"`
	User          User      `gorm:"constraint:OnDelete:CASCADE"                    json:"-"`
	CategoryID    *string   `gorm:"type:uuid;index"                                json:"category_id"`
	Category      *Category `gorm:"constraint:OnDelete:SET NULL"                   json:"-"`
	Type          string    `gorm:"size:20;not null;default:password_entry"        json:"type"`
	EncryptedBlob string    `gorm:"column:encrypted_blob;not null"                 json:"encrypted_blob"`
	IV            string    `gorm:"column:iv;not null"                             json:"iv"`
	Version       int       `gorm:"not null;default:1"                             json:"version"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (Vault) TableName() string { return "vault" }
