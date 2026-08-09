package db

import (
	"log"
	"time"

	"gorm.io/gorm"

	"librelock-server/models"
)

// How often expired session rows are swept
// Sessions are already rejected by the auth middleware the moment they expire, so this is housekeeping, not enforcement
const sessionSweepInterval = 15 * time.Minute

// PurgeExpiredSessions deletes session rows past their expiry and returns how many went
// A browser that is simply closed never reaches /auth/logout, so without this the table only grows
func PurgeExpiredSessions(database *gorm.DB) int64 {
	res := database.Where("expires_at <= ?", time.Now()).Delete(&models.Session{})
	if res.Error != nil {
		log.Printf("session purge: %v", res.Error)
		return 0
	}
	return res.RowsAffected
}

// StartSessionSweeper purges once at boot, then on a ticker for the life of the process
func StartSessionSweeper(database *gorm.DB) {
	if n := PurgeExpiredSessions(database); n > 0 {
		log.Printf("purged %d expired session(s)", n)
	}
	go func() {
		for range time.Tick(sessionSweepInterval) {
			PurgeExpiredSessions(database)
		}
	}()
}
