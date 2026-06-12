package models

import "time"

type User struct {
	ID             string    `gorm:"primaryKey;type:uuid;default:gen_random_uuid()" json:"id"`
	Username       string    `gorm:"uniqueIndex;size:200;not null"                  json:"username"`
	AuthHash       string    `gorm:"not null"                                       json:"-"`
	KDFAlgo        string    `gorm:"column:kdf_algo;size:50;not null;default:argon2id" json:"kdf_algo"`
	KDFSalt        string    `gorm:"column:kdf_salt;not null"                       json:"kdf_salt"`
	KDFIter        int       `gorm:"column:kdf_iter;not null"                       json:"kdf_iter"`
	KDFMemory      int       `gorm:"column:kdf_memory;not null"                     json:"kdf_memory"`
	KDFParallelism int       `gorm:"column:kdf_parallelism;not null"                json:"kdf_parallelism"`
	ProtectedKey   string    `gorm:"column:protected_key;not null"                  json:"protected_key"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (User) TableName() string { return "user" }
