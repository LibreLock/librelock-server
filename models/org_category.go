package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OrgCategory is a category for shared entries Tt has no owning user: it belongs to the organization The name is stored encrypted with the org key (server never sees plaintext) Only admins may create, rename, or delete; any member with shared access may read
type OrgCategory struct {
	ID        string    `gorm:"primaryKey;type:text" json:"id"`
	Name      string    `gorm:"not null"             json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (OrgCategory) TableName() string { return "org_category" }

func (c *OrgCategory) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	return nil
}
