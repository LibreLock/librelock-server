package db

import (
	"log"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"librelock-server/models"
)

func Connect(dsn string) *gorm.DB {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
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
