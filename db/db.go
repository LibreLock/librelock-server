package db

import (
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"librelock-server/models"
)

func Connect(path string) *gorm.DB {
	if dir := filepath.Dir(path); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			log.Fatalf("db dir: %v", err)
		}
	}

	dsn := path + "?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		log.Fatalf("db connect: %v", err)
	}

	if err := db.AutoMigrate(
		&models.User{},
		&models.Category{},
		&models.Vault{},
		&models.Session{},
	); err != nil {
		log.Fatalf("db migrate: %v", err)
	}

	return db
}
