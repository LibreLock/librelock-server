package main

import (
	"net/http"
	"reflect"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/go-playground/validator/v10"

	"librelock-server/config"
	"librelock-server/db"
	"librelock-server/handlers"
	"librelock-server/middleware"
)

func main() {
	cfg := config.Load()
	database := db.Connect(cfg.DSN)

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
	r.SetTrustedProxies(nil)
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	r.Use(middleware.SecurityHeaders())
	r.Use(middleware.MaxBodySize(1 << 20)) // 1 MiB
	r.Use(middleware.CORS(cfg.AllowedOrigin))

	authH := handlers.NewAuthHandler(database, cfg.TokenTTL, cfg.AppEnv)
	vaultH := handlers.NewVaultHandler(database)
	categoryH := handlers.NewCategoryHandler(database)
	settingsH := handlers.NewSettingsHandler(database, cfg.AppEnv)
	sessionH := handlers.NewSessionHandler(database)

	authMW := middleware.Auth(database)

	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "API is running"})
	})

	auth := r.Group("/auth")
	{
		auth.GET("/kdf", authH.KDF)
		auth.POST("/register", authH.Register)
		auth.POST("/login", authH.Login)
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

		s := protected.Group("/settings")
		s.PUT("/username", settingsH.UpdateUsername)
		s.PUT("/password", settingsH.UpdateMasterPassword)
		s.DELETE("/account", settingsH.DeleteAccount)

		sess := protected.Group("/sessions")
		sess.GET("", sessionH.Index)
		sess.DELETE("", sessionH.DestroyAll)
		sess.DELETE("/:id", sessionH.Destroy)
	}

	r.Run(":" + cfg.Port)
}
