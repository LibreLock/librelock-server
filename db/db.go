package db

import (
	"errors"
	"log"
	"os"
	"path/filepath"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"librelock-server/crypto"
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

	// Core tables only; org-only tables are added by MigrateOrg when in org mode
	if err := db.AutoMigrate(
		&models.User{},
		&models.Category{},
		&models.Vault{},
		&models.Session{},
		&models.AppState{},
	); err != nil {
		log.Fatalf("db migrate: %v", err)
	}

	return db
}

// MigrateOrg creates the organization-only tables
// Safe to call repeatedly
// The list must mirror appmode.EnableOrganization so a boot in org mode and a runtime switch produce the same schema
func MigrateOrg(db *gorm.DB) {
	if err := db.AutoMigrate(
		&models.Organization{},
		&models.Invite{},
		&models.AuditEvent{},
		&models.OrgVaultMembership{},
		&models.OrgCategory{},
		&models.OrgVault{},
	); err != nil {
		log.Fatalf("db migrate org: %v", err)
	}

	// Drop columns removed from models (AutoMigrate never drops)
	// Done with raw SQL because GORM's Migrator resolves the column against struct fields, which no longer exist once the field is deleted
	dropColumn(db, "organization", "primary_color")
}

// EnsureServerSecret returns this instance's random secret, generating it on first use
// It keys the decoy KDF salts, so it must survive restarts: regenerating per boot would make the same unknown username answer differently over time
// Call it after the mode is resolved - seeding app_state earlier would hide a legacy organization database from appmode's detection
func EnsureServerSecret(db *gorm.DB, mode string) string {
	var st models.AppState
	err := db.First(&st, "id = ?", models.AppStateSingletonID).Error
	if err == nil && st.ServerSecret != "" {
		return st.ServerSecret
	}

	secret := crypto.IssueToken()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := db.Create(&models.AppState{
			ID:           models.AppStateSingletonID,
			Mode:         mode,
			ServerSecret: secret,
		}).Error; err != nil {
			log.Fatalf("db server secret: %v", err)
		}
		return secret
	}
	if err != nil {
		log.Fatalf("db server secret: %v", err)
	}

	if err := db.Model(&models.AppState{}).
		Where("id = ?", models.AppStateSingletonID).
		UpdateColumn("server_secret", secret).Error; err != nil {
		log.Fatalf("db server secret: %v", err)
	}
	return secret
}

// EnsureOrgOwner promotes the oldest user to owner if the instance has none
func EnsureOrgOwner(db *gorm.DB) {
	var owners int64
	db.Model(&models.User{}).Where("role = ?", models.RoleOwner).Count(&owners)
	if owners > 0 {
		return
	}
	var user models.User
	if err := db.Order("created_at asc").First(&user).Error; err != nil {
		return // no users yet - first to register becomes owner
	}
	if err := db.Model(&user).UpdateColumn("role", models.RoleOwner).Error; err != nil {
		log.Printf("bootstrap: failed to promote owner: %v", err)
		return
	}
	log.Printf("bootstrap: promoted %q to owner", user.Username)
}

// dropColumn removes a column if the table and column exist (SQLite 3.35+)
func dropColumn(db *gorm.DB, table, column string) {
	var n int64
	db.Raw(
		"SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?",
		table, column,
	).Scan(&n)
	if n == 0 {
		return
	}
	if err := db.Exec("ALTER TABLE " + table + " DROP COLUMN " + column).Error; err != nil {
		log.Fatalf("db drop %s.%s: %v", table, column, err)
	}
}
