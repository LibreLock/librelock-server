package models

import (
	"time"
)

// OrgSingletonID is the fixed primary key of the single organization row
// (LibreLock is one-instance-per-company, not multi-tenant).
const OrgSingletonID = "org"

// Registration policies, admin-toggled at runtime on the org row.
const (
	RegistrationOpen   = "open"
	RegistrationInvite = "invite"
)

type Organization struct {
	ID           string    `gorm:"primaryKey;type:text"        json:"id"`
	Name         string    `gorm:"size:200;not null;default:LibreLock" json:"name"`
	LogoData     []byte    `gorm:"type:blob"                   json:"-"`
	LogoMimeType string    `gorm:"size:100"                    json:"-"`
	SupportEmail string    `gorm:"size:200"                    json:"support_email"`
	SupportURL   string    `gorm:"size:300"                    json:"support_url"`
	LoginMessage string    `gorm:"size:500"                    json:"login_message"`
	Registration string    `gorm:"size:20;not null;default:invite" json:"-"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (Organization) TableName() string { return "organization" }

func (o *Organization) HasLogo() bool { return len(o.LogoData) > 0 }
