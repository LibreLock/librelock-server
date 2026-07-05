package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Category struct {
	ID        string    `gorm:"primaryKey;type:text"                           json:"id"`
	UserID    string    `gorm:"type:text;not null;index"                       json:"user_id"`
	User      User      `gorm:"constraint:OnDelete:CASCADE"                    json:"-"`
	Name      string    `gorm:"not null"                                       json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Category) TableName() string { return "category" }

func (c *Category) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	return nil
}
