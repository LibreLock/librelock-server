package models

import "time"

const AppStateSingletonID = "app"

// AppState is the singleton row that persists the deployment mode
type AppState struct {
	ID   string `gorm:"primaryKey;type:text"                json:"-"`
	Mode string `gorm:"size:20;not null;default:personal"   json:"mode"`
	// Random per-instance secret, keyed into the decoy KDF salts handed out for unknown usernames
	// Persisted rather than generated per boot so those salts stay stable across restarts
	ServerSecret string `gorm:"size:64;not null;default:''"      json:"-"`
	// Personal-mode sign-up switch: off once the instance has its first account, so a personal
	// instance never accepts strangers unless its owner opts in from Settings
	// Organization mode ignores this and uses organization.registration instead
	AllowRegistration bool      `gorm:"not null;default:false"        json:"-"`
	CreatedAt         time.Time `json:"-"`
	UpdatedAt         time.Time `json:"-"`
}

func (AppState) TableName() string { return "app_state" }
