package db

import (
	"log"

	"gorm.io/gorm"

	"librelock-server/models"
	"librelock-server/version"
)

// migration is one step AutoMigrate cannot express: a rename, a backfill, a drop, a row rewrite
// AutoMigrate only ever diffs the current models against the current tables, so it has no way to
// move data, and no memory of what it has already done
type migration struct {
	name string
	run  func(*gorm.DB) error
}

// migrations is append-only: the index is the schema version, so reordering or deleting an entry
// renumbers every database in the wild. Add to the end, never edit an entry that has shipped
//
// Each step must be safe on a personal database too - the org-only tables do not exist there - and
// must tolerate running against data written by any older release, since an instance can jump
// several versions in one upgrade
var migrations = []migration{
	{
		name: "drop organization.primary_color",
		run: func(tx *gorm.DB) error {
			return dropColumn(tx, "organization", "primary_color")
		},
	},
}

// RunMigrations applies everything the database has not seen yet and records how far it got
// Call it after AutoMigrate has built the tables and after the app_state row exists
func RunMigrations(db *gorm.DB, boot Bootstrap) {
	head := len(migrations)

	// A database created by this boot is already at head: AutoMigrate built every table from the
	// current models, and there are no rows for a data migration to fix up. Stamping it here is what
	// stops historical steps from running against a brand-new install
	if !boot.Existing {
		stamp(db, head)
		return
	}

	// Connect already refused this boot; kept here because this is the function that owns the invariant
	guardSchemaVersion(boot)

	for i := boot.SchemaVersion; i < head; i++ {
		m := migrations[i]
		// SQLite is one of the few engines with transactional DDL, so a step that fails partway
		// leaves nothing behind - including the version bump, which rides in the same transaction
		if err := db.Transaction(func(tx *gorm.DB) error {
			if err := m.run(tx); err != nil {
				return err
			}
			return setSchemaVersion(tx, i+1)
		}); err != nil {
			log.Fatalf("migration %d (%s): %v", i+1, m.name, err)
		}
		log.Printf("db migrated to schema %d: %s", i+1, m.name)
	}

	stamp(db, head)
}

// guardSchemaVersion stops a database that a newer release has migrated from being opened by an
// older binary: the schema has moved under code that does not know about it, and every write from
// here on is guesswork. Nothing has been modified at the point this runs, so there is nothing to undo
func guardSchemaVersion(boot Bootstrap) {
	if boot.SchemaVersion <= len(migrations) {
		return
	}
	log.Fatalf("database is at schema %d but this build (%s) only knows %d:"+
		" it was migrated by a newer LibreLock. Run that version again, or restore a snapshot from the %s directory",
		boot.SchemaVersion, version.Version, len(migrations), backupDirName)
}

// stamp records the schema version and the release that produced it
// AppVersion is what the next boot compares against to decide whether it is an upgrade worth snapshotting
func stamp(db *gorm.DB, schemaVersion int) {
	res := db.Model(&models.AppState{}).
		Where("id = ?", models.AppStateSingletonID).
		UpdateColumns(map[string]any{
			"schema_version": schemaVersion,
			"app_version":    version.Version,
		})
	if res.Error != nil {
		log.Fatalf("db stamp version: %v", res.Error)
	}
	// The row is seeded by EnsureServerSecret, which runs first: no row means the boot order broke
	if res.RowsAffected == 0 {
		log.Fatalf("db stamp version: no app_state row - RunMigrations ran before the row was seeded")
	}
}

func setSchemaVersion(tx *gorm.DB, v int) error {
	return tx.Model(&models.AppState{}).
		Where("id = ?", models.AppStateSingletonID).
		UpdateColumn("schema_version", v).Error
}
