// Package appmode holds the live deployment mode (personal vs organization), persisted in the database so it can be switched on a running instance
package appmode

import (
	"errors"
	"sync"

	"gorm.io/gorm"

	"librelock-server/config"
	"librelock-server/models"
)

type Provider struct {
	db      *gorm.DB
	mu      sync.RWMutex
	mode    string
	openReg bool
}

func New(db *gorm.DB) *Provider {
	p := &Provider{db: db, mode: config.ModePersonal}

	var st models.AppState
	err := db.First(&st, "id = ?", models.AppStateSingletonID).Error
	switch {
	case err == nil:
		if st.Mode == config.ModeOrganization {
			p.mode = config.ModeOrganization
		}
		p.openReg = st.AllowRegistration
	case db.Migrator().HasTable("organization"):
		// Legacy org DB from before app_state existed: adopt org mode once
		p.mode = config.ModeOrganization
		p.persist(config.ModeOrganization)
	}

	return p
}

func (p *Provider) Current() string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mode
}

func (p *Provider) IsOrganization() bool {
	return p.Current() == config.ModeOrganization
}

// RegistrationOpen reports whether a personal instance accepts new sign-ups
// It is meaningless in organization mode, which reads organization.registration instead
func (p *Provider) RegistrationOpen() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.openReg
}

// SetRegistrationOpen persists the personal-mode sign-up switch
func (p *Provider) SetRegistrationOpen(open bool) error {
	if err := p.db.Model(&models.AppState{}).
		Where("id = ?", models.AppStateSingletonID).
		UpdateColumn("allow_registration", open).Error; err != nil {
		return err
	}
	p.mu.Lock()
	p.openReg = open
	p.mu.Unlock()
	return nil
}

// EnableOrganization creates the org-only tables, makes the acting user the owner, seeds the branding row, and persists the mode
// No-op if already org
func (p *Provider) EnableOrganization(actorID string) error {
	if p.IsOrganization() {
		return nil
	}

	// Resolve the actor before touching the schema: an organization with no owner can neither be
	// administered nor reverted, since both are owner-gated, so a switch that cannot produce one
	// must leave no trace
	if actorID != "" {
		if err := p.db.First(&models.User{}, "id = ?", actorID).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("appmode: actor not found, refusing to enable organization mode without an owner")
			}
			return err
		}
	}

	if err := p.db.AutoMigrate(
		&models.Organization{},
		&models.Invite{},
		&models.AuditEvent{},
		&models.OrgVaultMembership{},
		&models.OrgCategory{},
		&models.OrgVault{},
	); err != nil {
		return err
	}

	// The owner promotion must succeed before the mode flips
	if actorID != "" {
		res := p.db.Model(&models.User{}).Where("id = ?", actorID).
			UpdateColumn("role", models.RoleOwner)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return errors.New("appmode: owner promotion affected no rows")
		}
		// An org can hold several owners, but the account enabling it is the only one at bootstrap;
		// demote any owner left over from an earlier organization phase
		if err := p.db.Model(&models.User{}).
			Where("id <> ? AND role = ?", actorID, models.RoleOwner).
			UpdateColumn("role", models.RoleAdmin).Error; err != nil {
			return err
		}
	}

	var org models.Organization
	if err := p.db.First(&org, "id = ?", models.OrgSingletonID).Error; err == gorm.ErrRecordNotFound {
		p.db.Create(&models.Organization{
			ID:           models.OrgSingletonID,
			Name:         "LibreLock",
			Registration: models.RegistrationInvite,
		})
	}

	if err := p.persist(config.ModeOrganization); err != nil {
		return err
	}

	p.mu.Lock()
	p.mode = config.ModeOrganization
	p.mu.Unlock()
	return nil
}

// RevertToPersonal drops the org-only tables and persists personal mode
// It does not delete users; the caller removes accounts first
// No-op if already personal
func (p *Provider) RevertToPersonal() error {
	if !p.IsOrganization() {
		return nil
	}

	if err := p.persist(config.ModePersonal); err != nil {
		return err
	}
	// The revert leaves a single account behind, so close sign-up again rather than inheriting
	// whatever the organization's registration policy was
	if err := p.SetRegistrationOpen(false); err != nil {
		return err
	}

	for _, table := range []string{"org_vault", "org_category", "org_vault_membership", "audit_event", "invite", "organization"} {
		if err := p.db.Exec("DROP TABLE IF EXISTS " + table).Error; err != nil {
			return err
		}
	}

	p.mu.Lock()
	p.mode = config.ModePersonal
	p.mu.Unlock()
	return nil
}

// persist writes the mode column only
// A full Save of the struct would blank the other singleton columns (server secret, registration switch)
func (p *Provider) persist(mode string) error {
	res := p.db.Model(&models.AppState{}).
		Where("id = ?", models.AppStateSingletonID).
		UpdateColumn("mode", mode)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return p.db.Create(&models.AppState{ID: models.AppStateSingletonID, Mode: mode}).Error
	}
	return nil
}
