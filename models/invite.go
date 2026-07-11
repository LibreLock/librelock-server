package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Invite is a single-use registration token for organization invite modew
// Only the SHA-256 hash of the token is stored; the raw token is shown once
type Invite struct {
	ID        string     `gorm:"primaryKey;type:text"          json:"id"`
	TokenHash string     `gorm:"uniqueIndex;not null"          json:"-"`
	Note      string     `gorm:"size:200"                      json:"note"`
	CreatedBy string     `gorm:"type:text;index"               json:"created_by"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at"`
	CreatedAt time.Time  `json:"created_at"`
}

func (Invite) TableName() string { return "invite" }

func (i *Invite) BeforeCreate(tx *gorm.DB) error {
	if i.ID == "" {
		i.ID = uuid.NewString()
	}
	return nil
}

func (i *Invite) IsUsed() bool    { return i.UsedAt != nil }
func (i *Invite) IsExpired() bool { return time.Now().After(i.ExpiresAt) }
func (i *Invite) IsValid() bool   { return !i.IsUsed() && !i.IsExpired() }
