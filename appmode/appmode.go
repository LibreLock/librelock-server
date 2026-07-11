// Package appmode holds the live deployment mode (personal vs organization), persisted in the database so it can be switched on a running instance
package appmode

import (
	"sync"

	"gorm.io/gorm"

	"librelock-server/config"
	"librelock-server/models"
)

type Provider struct {
	db   *gorm.DB
	mu   sync.RWMutex
	mode string
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

// EnableOrganization creates the org-only tables, makes the acting user the owner, seeds the branding row, and persists the mode
// No-op if already org
func (p *Provider) EnableOrganization(actorID string) error {
	if p.IsOrganization() {
		return nil
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

	if actorID != "" {
		p.db.Model(&models.User{}).Where("id = ?", actorID).
			UpdateColumn("role", models.RoleOwner)
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

func (p *Provider) persist(mode string) error {
	st := models.AppState{ID: models.AppStateSingletonID, Mode: mode}
	return p.db.Save(&st).Error
}
