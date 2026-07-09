package models

import "time"

const AppStateSingletonID = "app"

// AppState is the singleton row that persists the deployment mode.
type AppState struct {
	ID        string    `gorm:"primaryKey;type:text"                json:"-"`
	Mode      string    `gorm:"size:20;not null;default:personal"   json:"mode"`
	CreatedAt time.Time `json:"-"`
	UpdatedAt time.Time `json:"-"`
}

func (AppState) TableName() string { return "app_state" }
