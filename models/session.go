package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Session struct {
	ID         string    `gorm:"primaryKey;type:text"                           json:"id"`
	UserID     string    `gorm:"type:text;not null;index"                       json:"user_id"`
	User       User      `gorm:"constraint:OnDelete:CASCADE"                    json:"-"`
	TokenHash  string    `gorm:"column:token_hash;size:64;not null;index"       json:"-"`
	DeviceName *string   `gorm:"column:device_name;size:255"                    json:"device_name"`
	IP         string    `gorm:"size:45;not null"                               json:"ip"`
	CreatedAt  time.Time `gorm:"autoCreateTime"                                 json:"created_at"`
	LastUsedAt time.Time `gorm:"column:last_used_at;autoCreateTime"             json:"last_used_at"`
	ExpiresAt  time.Time `gorm:"column:expires_at;not null"                     json:"expires_at"`
}

func (Session) TableName() string { return "session" }

func (s *Session) BeforeCreate(tx *gorm.DB) error {
	if s.ID == "" {
		s.ID = uuid.NewString()
	}
	return nil
}
