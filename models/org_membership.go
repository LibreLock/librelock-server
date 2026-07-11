package models

import "time"

// OrgVaultMembership grants a user access to the shared organization vault
// WrappedKey is the shared org key enveloped to this user's public key, so the server stores it without ever seeing the key itself
// One row per user
type OrgVaultMembership struct {
	UserID     string    `gorm:"primaryKey;type:text"        json:"user_id"`
	User       User      `gorm:"constraint:OnDelete:CASCADE" json:"-"`
	WrappedKey string    `gorm:"column:wrapped_key;not null" json:"wrapped_key"`
	GrantedBy  string    `gorm:"type:text"                   json:"granted_by"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

func (OrgVaultMembership) TableName() string { return "org_vault_membership" }
