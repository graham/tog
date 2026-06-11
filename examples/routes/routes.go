package routes

import (
	"log"
	"os"

	"github.com/go-chi/chi/v5"
	"github.com/graham/tog/app"
	"github.com/graham/tog/db"
	"github.com/graham/tog/examples/email"
	"github.com/graham/tog/examples/routes/dev"
	"github.com/graham/tog/examples/routes/items"
	"github.com/graham/tog/examples/routes/userlogs"
	"github.com/graham/tog/web/auth"
)

// LoadRoutes sets up application-specific routes.
// It receives a chi router with standard middleware already applied
// and health/docs endpoints mounted by the app package.
func LoadRoutes(r chi.Router, dbm *db.Manager) error {
	// Load app configuration
	appConfig, err := app.LoadAppConfig("")
	if err != nil {
		log.Printf("Warning: failed to load config.json, using defaults: %v", err)
		appConfig = app.DefaultAppConfig()
	}

	// Register all queries
	itemQueries, err := items.RegisterQueries(dbm.Default())
	if err != nil {
		return err
	}

	userlogQueries, err := userlogs.RegisterQueries(dbm.Default())
	if err != nil {
		return err
	}

	authQueries, err := auth.RegisterQueries(dbm.Default())
	if err != nil {
		return err
	}

	// Check dev mode from environment
	devMode := os.Getenv("ENVIRONMENT") == "dev"

	// Protected API routes (require authentication)
	itemRoutes := items.NewRoutes(itemQueries)
	userlogRoutes := userlogs.NewRoutes(userlogQueries)
	r.Group(func(r chi.Router) {
		r.Use(auth.RequiresAuth(authQueries))
		r.Route("/api/items", itemRoutes.Mount("/api/items"))
		r.Route("/api/logs", userlogRoutes.Mount("/api/logs"))
	})

	// Public items route (no auth, for testing timers)
	r.Route("/api/noauth/items", itemRoutes.MountPublic("/api/noauth/items"))

	// Auth routes with full configuration
	// Configure email sender if Resend API key is set in config
	var emailSender auth.MagicLinkEmailSender
	if appConfig.Email.ResendAPIKey != "" {
		emailSender = email.NewResendSender(
			appConfig.Email.ResendAPIKey,
			appConfig.Email.FromAddress,
			appConfig.Email.FromName,
		)
	}

	var authRoutes *auth.Routes
	if emailSender != nil {
		authRoutes = auth.NewRoutesWithEmail(authQueries, devMode, auth.DefaultCookieConfig(), &appConfig.Auth, emailSender)
	} else {
		authRoutes = auth.NewRoutesWithAuth(authQueries, devMode, auth.DefaultCookieConfig(), &appConfig.Auth)
	}
	r.Route("/auth", authRoutes.Mount())

	// Dev routes (only in dev environment)
	if os.Getenv("ENVIRONMENT") == "dev" {
		devRoutes := dev.NewRoutes(dbm)
		r.Route("/dev", devRoutes.Mount())
	}

	return nil
}
