package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// AuditEvent records an admin action Actor/target names are snapshotted so entries survive the referenced user being deleted
type AuditEvent struct {
	ID         string    `gorm:"primaryKey;type:text"    json:"id"`
	Action     string    `gorm:"size:60;not null;index"  json:"action"`
	ActorID    string    `gorm:"type:text;index"         json:"actor_id"`
	ActorName  string    `gorm:"size:200"                json:"actor_name"`
	TargetID   string    `gorm:"type:text"               json:"target_id"`
	TargetName string    `gorm:"size:200"                json:"target_name"`
	Detail     string    `gorm:"size:300"                json:"detail"`
	CreatedAt  time.Time `gorm:"index"                   json:"created_at"`
}

func (AuditEvent) TableName() string { return "audit_event" }

func (a *AuditEvent) BeforeCreate(tx *gorm.DB) error {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	return nil
}
