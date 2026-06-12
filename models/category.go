package models

import "time"

type Category struct {
	ID        string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	UserID    string    `gorm:"type:uuid;not null;index"                       json:"user_id"`
	User      User      `gorm:"constraint:OnDelete:CASCADE"                    json:"-"`
	Name      string    `gorm:"not null"                                       json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Category) TableName() string { return "category" }
