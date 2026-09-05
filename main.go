package main

import (
	"log"
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"librelock-server/appmode"
	"librelock-server/config"
	"librelock-server/db"
	"librelock-server/handlers"
	"librelock-server/middleware"
	"librelock-server/version"
)

func main() {
	cfg := config.Load()
	database, boot := db.Connect(cfg.DBPath, cfg.UpgradeBackups)

	// Mode is persisted in the database and read live via the provider
	mode := appmode.New(database)
	// After appmode.New: seeding app_state any earlier would mask a legacy organization database
	serverSecret := db.EnsureServerSecret(database, mode.Current())
	if mode.IsOrganization() {
		db.MigrateOrg(database)
		db.EnsureOrgOwner(database)
	}
	// Last, so every table exists and the app_state row is there to record how far it got
	db.RunMigrations(database, boot)
	db.StartSessionSweeper(database)
	log.Printf("librelock %s mode=%s", version.Version, mode.Current())

	// Use JSON tag names in validation error messages
	if v, ok := binding.Validator.Engine().(*validator.Validate); ok {
		v.RegisterTagNameFunc(func(fld reflect.StructField) string {
			name := strings.SplitN(fld.Tag.Get("json"), ",", 2)[0]
			if name == "-" {
				return ""
			}
			return name
		})
	}

	if cfg.AppEnv == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	if err := r.SetTrustedProxies(cfg.TrustedProxies); err != nil {
		log.Fatalf("trusted proxies: %v", err)
	}
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.MaxBodySize(1 << 20)) // 1 MiB
	r.Use(middleware.CORS(cfg.AllowedOrigin))

	authH := handlers.NewAuthHandler(database, cfg.TokenTTL, cfg.AppEnv, mode, serverSecret)
	vaultH := handlers.NewVaultHandler(database)
	orgVaultH := handlers.NewOrgVaultHandler(database)
	orgCategoryH := handlers.NewOrgCategoryHandler(database)
	categoryH := handlers.NewCategoryHandler(database)
	settingsH := handlers.NewSettingsHandler(database, cfg.AppEnv, mode)
	sessionH := handlers.NewSessionHandler(database)
	orgH := handlers.NewOrganizationHandler(database, mode)

	authMW := middleware.Auth(database, cfg.TokenTTL)

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "API is running"})
	})

	r.GET("/version", handlers.Version)

	// Public branding
	// The frontend swaps logo/name on load
	r.GET("/organization", orgH.Show)
	r.GET("/organization/logo", orgH.Logo)

	// kdf invites an enumeration sweep, login and register each cost a 64 MiB argon2 hash, and a real sign-in spends two requests
	// /auth/me and /auth/logout stay out: they are authenticated, hash nothing, and /auth/me runs on every page load, so sharing a bucket would only 429 real users behind one egress IP
	authLimit := middleware.RateLimit(20, 1.0/3.0)
	auth := r.Group("/auth")
	{
		auth.GET("/kdf", authLimit, authH.KDF)
		auth.POST("/register", authLimit, authH.Register)
		auth.POST("/login", authLimit, authH.Login)
		auth.POST("/logout", authMW, authH.Logout)
		auth.GET("/me", authMW, authH.Me)
	}

	protected := r.Group("/", authMW)
	{
		v := protected.Group("/vault")
		v.GET("", vaultH.Index)
		v.POST("", vaultH.Store)
		v.GET("/:id", vaultH.Show)
		v.PUT("/:id", vaultH.Update)
		v.DELETE("/:id", vaultH.Destroy)

		cat := protected.Group("/categories")
		cat.GET("", categoryH.Index)
		cat.POST("", categoryH.Store)
		cat.GET("/:id", categoryH.Show)
		cat.PUT("/:id", categoryH.Update)
		cat.DELETE("/:id", categoryH.Destroy)

		// These three re-authenticate the caller, so each one spends a 64 MiB argon2 hash before it can reject a wrong password
		// Unlimited, a single session can hold every hash slot (crypto.hashSlots) and stall sign-in for the whole instance, so they get their own bucket
		// The rest of the group hashes nothing and stays unmetered
		reauthLimit := middleware.RateLimit(5, 1.0/10.0)

		s := protected.Group("/settings")
		s.PUT("/username", settingsH.UpdateUsername)
		s.PUT("/theme", settingsH.UpdateTheme)
		s.PUT("/password", reauthLimit, settingsH.UpdateMasterPassword)
		s.PUT("/keypair", settingsH.UploadKeypair)
		s.DELETE("/account", reauthLimit, settingsH.DeleteAccount)
		s.PUT("/mode", reauthLimit, settingsH.SwitchMode)
		// Instance-wide settings, personal mode only (organization mode covers these in its own area)
		s.GET("/instance", settingsH.ShowInstance)
		s.PUT("/registration", settingsH.UpdateRegistration)

		sess := protected.Group("/sessions")
		sess.GET("", sessionH.Index)
		sess.DELETE("", sessionH.DestroyAll)
		sess.DELETE("/:id", sessionH.Destroy)

		// Member-facing: fetch own wrapped shared-vault key
		protected.GET("/org/shared-key", orgH.MyOrgKey)

		ov := protected.Group("/org-vault", middleware.RequireMembership(database, mode))
		ov.GET("", orgVaultH.Index)
		ov.POST("", orgVaultH.Store)
		ov.GET("/:id", orgVaultH.Show)
		ov.PUT("/:id", orgVaultH.Update)
		ov.DELETE("/:id", orgVaultH.Destroy)

		// Shared categories: any member reads; only admins write (checked in the handler, since writers also need membership to hold the org key)
		oc := protected.Group("/org-categories", middleware.RequireMembership(database, mode))
		oc.GET("", orgCategoryH.Index)
		oc.POST("", orgCategoryH.Store)
		oc.GET("/:id", orgCategoryH.Show)
		oc.PUT("/:id", orgCategoryH.Update)
		oc.DELETE("/:id", orgCategoryH.Destroy)

		org := protected.Group("/organization", middleware.RequireAdmin(mode))
		org.PUT("", orgH.Update)
		org.PUT("/registration", orgH.UpdateRegistration)
		org.PUT("/shared-settings", orgH.UpdateSharedSettings)
		org.PUT("/logo", orgH.UploadLogo)
		org.DELETE("/logo", orgH.DeleteLogo)
		org.GET("/users", orgH.ListUsers)
		org.PUT("/users/:id/role", orgH.UpdateUserRole)
		org.PUT("/users/:id/status", orgH.UpdateUserStatus)
		org.DELETE("/users/:id", orgH.RemoveUser)
		org.POST("/invites", orgH.CreateInvite)
		org.GET("/invites", orgH.ListInvites)
		org.DELETE("/invites", orgH.PruneInvites)
		org.DELETE("/invites/:id", orgH.RevokeInvite)
		org.GET("/audit", orgH.ListAuditEvents)
		org.GET("/memberships", orgH.ListMemberships)
		org.POST("/memberships", orgH.GrantMembership)
		org.DELETE("/memberships/:userId", orgH.RevokeMembership)
	}

	r.Run(":" + cfg.Port)
}
