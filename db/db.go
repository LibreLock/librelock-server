package db

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"librelock-server/crypto"
	"librelock-server/models"
	"librelock-server/version"
)

const (
	// Snapshots live beside the database, inside the same volume, so a backup of the data directory carries them
	backupDirName = "backups"
	// Enough to step back through a bad upgrade without letting an unattended instance grow forever
	backupsKept = 3
)

// Bootstrap is what the database looked like before this boot touched it
// RunMigrations needs it after the app_state row is guaranteed to exist, which is later in the boot
type Bootstrap struct {
	// Core tables were already there, ie. this is not a first run
	Existing bool
	// Release that last ran against the file; empty on databases older than the column
	AppVersion string
	// Migration list index already applied
	SchemaVersion int
}

// Connect opens the database, snapshots it when this boot is an upgrade, and creates or extends the
// core tables. backups=false skips the snapshot, for installs where the disk cannot hold a second copy
func Connect(path string, backups bool) (*gorm.DB, Bootstrap) {
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

	boot := inspect(db)

	// Before anything writes: AutoMigrate would happily re-add columns a newer release dropped,
	// leaving stray ones behind that no migration will ever clean up again
	guardSchemaVersion(boot)

	// Snapshot before the schema moves: a migration that goes wrong is unrecoverable for an
	// end-to-end-encrypted vault - the server holds no plaintext to rebuild it from - and the
	// "back up first" line in the docs is read by approximately nobody
	if backups && boot.Existing && boot.AppVersion != version.Version {
		snapshot(db, path, boot.AppVersion)
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

	return db, boot
}

// inspect reads the pre-boot state without creating the app_state row: seeding it here would mask a
// legacy organization database from appmode's detection, which keys off the row being absent
func inspect(db *gorm.DB) Bootstrap {
	boot := Bootstrap{Existing: db.Migrator().HasTable("user")}

	// The columns arrive with this feature, so anything older reads as schema 0 and migrates from the start
	if !hasColumn(db, "app_state", "schema_version") {
		return boot
	}

	var row struct {
		AppVersion    string
		SchemaVersion int
	}
	db.Raw(
		"SELECT COALESCE(app_version, '') AS app_version, COALESCE(schema_version, 0) AS schema_version"+
			" FROM app_state WHERE id = ?",
		models.AppStateSingletonID,
	).Scan(&row)

	boot.AppVersion = row.AppVersion
	boot.SchemaVersion = row.SchemaVersion
	return boot
}

// snapshot copies the database next to it, under the version it is being upgraded from
func snapshot(db *gorm.DB, path, from string) {
	dir := filepath.Join(filepath.Dir(path), backupDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		log.Fatalf("db backup dir: %v", err)
	}

	if from == "" {
		from = "unknown"
	}
	name := fmt.Sprintf("%s-%s-%s.db",
		strings.TrimSuffix(filepath.Base(path), filepath.Ext(path)),
		safeForFilename(from),
		time.Now().UTC().Format("20060102-150405"),
	)
	dst := filepath.Join(dir, name)

	// VACUUM INTO folds the WAL into one consistent file on the live connection: a plain copy would
	// miss it and leave the -wal sidecar behind, and the runtime image carries no sqlite3 binary
	if err := db.Exec("VACUUM INTO ?", dst).Error; err != nil {
		log.Fatalf("pre-upgrade backup to %s failed: %v"+
			" (free space for a second copy of the database, or set UPGRADE_BACKUPS=false to skip it)", dst, err)
	}
	log.Printf("pre-upgrade backup: %s", dst)

	pruneBackups(dir)
}

// pruneBackups keeps the newest few snapshots so an unattended instance cannot fill its disk
func pruneBackups(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	type snap struct {
		path string
		mod  time.Time
	}
	var snaps []snap
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".db" {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		snaps = append(snaps, snap{filepath.Join(dir, e.Name()), info.ModTime()})
	}
	if len(snaps) <= backupsKept {
		return
	}

	sort.Slice(snaps, func(i, j int) bool { return snaps[i].mod.After(snaps[j].mod) })
	for _, s := range snaps[backupsKept:] {
		if err := os.Remove(s.path); err != nil {
			log.Printf("db backup prune: %v", err)
		}
	}
}

// safeForFilename keeps version strings like 0.1.0 or main-2807603 intact and neutralises anything else
func safeForFilename(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '.', r == '-', r == '_':
			return r
		default:
			return '-'
		}
	}, s)
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
// Raw SQL because GORM's Migrator resolves the column against struct fields, which no longer exist once the field is deleted
// The existence check is what makes it safe on a personal database, where the org tables were never created
func dropColumn(db *gorm.DB, table, column string) error {
	if !hasColumn(db, table, column) {
		return nil
	}
	return db.Exec("ALTER TABLE " + table + " DROP COLUMN " + column).Error
}

func hasColumn(db *gorm.DB, table, column string) bool {
	var n int64
	db.Raw(
		"SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = ?",
		table, column,
	).Scan(&n)
	return n > 0
}
