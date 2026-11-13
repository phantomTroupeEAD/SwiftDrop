package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	_ "github.com/lib/pq"
	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
	"github.com/swiftdrop/core-api/internal/identity/infrastructure"
	"github.com/swiftdrop/core-api/internal/identity/transport"
	"github.com/swiftdrop/core-api/internal/platform/config"
	"github.com/swiftdrop/core-api/migrations"
)

// AppContainer holds all application services, database handles, and HTTP handlers.
type AppContainer struct {
	Mux         *http.ServeMux
	AuthSvc     *application.AuthService
	UserSvc     *application.UserService
	UserRepo    application.UserRepository
	AuditWriter application.AuditWriter
	Security    *transport.SecurityMiddleware
	TokenProv   application.TokenProvider
	DB          *sql.DB
	StorageType string
}

// BuildAppContainer wires all dependencies and sets up HTTP routes.
// It connects to PostgreSQL, automatically applies migrations, and wires real infrastructure adapters.
func BuildAppContainer() *AppContainer {
	config.LoadEnv(".env", "../.env", "../../.env")

	// 1. Password Hasher & JWT Token Provider
	hasher := infrastructure.NewStandardPasswordHasher()
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		log.Println("[WARN] JWT_SECRET environment variable is not set.")
	}
	tokenProvider := infrastructure.NewJWTTokenProvider(jwtSecret)

	var userRepo application.UserRepository
	var tokenRepo application.TokenRepository
	var auditWriter application.AuditWriter
	var oauthRepo application.OAuthRepository
	var otpRepo application.OTPRepository
	var dbConn *sql.DB
	storageType := "PostgreSQL (Disconnected)"

	// 2. Connect to PostgreSQL
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL != "" {
		db, err := sql.Open("postgres", dbURL)
		if err == nil {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			if pingErr := db.PingContext(ctx); pingErr == nil {
				log.Println("[INFO] Successfully connected to PostgreSQL database.")

				if migErr := migrations.RunMigrations(context.Background(), db); migErr != nil {
					log.Printf("[WARN] Auto-migration error: %v\n", migErr)
				}

				userRepo = infrastructure.NewPostgresUserRepository(db)
				tokenRepo = infrastructure.NewPostgresTokenRepository(db)
				auditWriter = infrastructure.NewPostgresAuditWriter(db)
				oauthRepo = infrastructure.NewPostgresOAuthRepository(db)
				otpRepo = infrastructure.NewPostgresOTPRepository(db)
				dbConn = db
				storageType = "PostgreSQL Database (Authoritative)"
			} else {
				_ = db.Close()
				log.Printf("[ERROR] PostgreSQL is unreachable at %s: %v\n", dbURL, pingErr)
			}
		} else {
			log.Printf("[ERROR] Invalid DATABASE_URL: %v\n", err)
		}
	} else {
		log.Println("[ERROR] DATABASE_URL environment variable is not set.")
	}

	// 3. Email Sender Adapter (SMTP required for transactional emails)
	var emailSender application.EmailSender
	smtpHost := os.Getenv("SMTP_HOST")
	if smtpHost != "" {
		smtpPort := os.Getenv("SMTP_PORT")
		if smtpPort == "" {
			smtpPort = "587"
		}
		smtpUser := os.Getenv("SMTP_USER")
		smtpPass := os.Getenv("SMTP_PASS")
		smtpFrom := os.Getenv("SMTP_FROM")
		if smtpFrom == "" {
			smtpFrom = "no-reply@swiftdrop.com"
		}
		emailSender = infrastructure.NewSMTPEmailSender(smtpHost, smtpPort, smtpUser, smtpPass, smtpFrom)
		log.Printf("[INFO] Using SMTP Email Sender (%s:%s)\n", smtpHost, smtpPort)
	} else {
		emailSender = infrastructure.NewUnconfiguredEmailSender()
		log.Println("[WARN] SMTP_HOST is not configured. Transactional OTP emails will fail until SMTP_HOST is set.")
	}

	// 4. Application Services
	googleClientID := os.Getenv("GOOGLE_CLIENT_ID")
	oidcClient := infrastructure.NewGoogleOIDCClient(googleClientID)

	authService := application.NewAuthService(userRepo, tokenRepo, hasher, tokenProvider).
		WithOTP(otpRepo, emailSender).
		WithOAuth(oauthRepo, oidcClient)

	// 5. Transport & Middleware
	security := transport.NewSecurityMiddleware(tokenProvider)
	authHandler := transport.NewAuthHandler(authService)

	mux := http.NewServeMux()

	// 6. Register Identity Routes
	transport.RegisterRoutes(mux, authHandler, security)
	userService := application.NewUserService(userRepo, auditWriter)
	adminHandler := transport.NewAdminHandler(userService)
	transport.RegisterAdminRoutes(mux, adminHandler, security)

	// 7. Role-Protected Test Endpoints for Teammates
	mux.Handle("GET /api/v1/courier/dashboard", security.Authenticate(
		transport.RequireRole(domain.RoleCourier, domain.RoleAdmin)(
			http.HandlerFunc(handleCourierDashboard),
		),
	))

	mux.Handle("GET /api/v1/merchant/portal", security.Authenticate(
		transport.RequireRole(domain.RoleMerchant, domain.RoleAdmin)(
			transport.RequireTenant()(
				http.HandlerFunc(handleMerchantPortal),
			),
		),
	))

	mux.Handle("GET /api/v1/admin/dashboard", security.Authenticate(
		transport.RequireRole(domain.RoleAdmin)(
			http.HandlerFunc(handleAdminDashboard),
		),
	))

	// Legacy Health Endpoint
	mux.HandleFunc("GET /api/v1/health", handleLegacyHealth(storageType))

	// Standard Kubernetes / Infrastructure Health Probes (Phase 8B)
	mux.HandleFunc("GET /health/live", handleHealthLive)
	mux.HandleFunc("GET /health/ready", handleHealthReady(dbConn))

	// 8. Register Member 08 Events & Messaging Operations & Swagger UI
	registerEventsModules(mux, dbConn, security)

	// 9. Register Member 04 Merchant Integration Operations
	if dbConn != nil {
		if err := registerMerchantsModule(mux, dbConn, tokenProvider, userService); err != nil {
			if config.IsProduction() {
				log.Fatalf("[FATAL] Failed to initialize Member 04 Merchant Integration: %v", err)
			}
			log.Printf("[NOTICE] Member 04 Merchant Integration not fully active in local mode: %v\n", err)
		}
	} else {
		log.Println("[NOTICE] Member 04 Merchant Integration not fully active: PostgreSQL connection (*sql.DB) is required")
	}

	// 10. Register Member 03 Dispatch & Delivery Modules
	registerDispatchAndDeliveryModules(mux, dbConn, security)

	return &AppContainer{
		Mux:         mux,
		AuthSvc:     authService,
		UserSvc:     userService,
		AuditWriter: auditWriter,
		UserRepo:    userRepo,
		Security:    security,
		TokenProv:   tokenProvider,
		DB:          dbConn,
		StorageType: storageType,
	}
}

// handleCourierDashboard godoc
// @Summary      Courier dashboard status
// @Description  Test dashboard endpoint for authenticated Couriers and Admins.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} transport.ErrorEnvelope "Unauthorized"
// @Router       /api/v1/courier/dashboard [get]
func handleCourierDashboard(w http.ResponseWriter, r *http.Request) {
	principal, _ := transport.GetPrincipal(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Welcome Courier! Your courier routes are active.",
		"user_id":   principal.UserID,
		"role":      principal.Role,
		"active_id": "route_colombo_central",
	})
}

// handleMerchantPortal godoc
// @Summary      Merchant portal status
// @Description  Test portal endpoint for authenticated Merchants and Admins with active tenant.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} transport.ErrorEnvelope "Unauthorized"
// @Router       /api/v1/merchant/portal [get]
func handleMerchantPortal(w http.ResponseWriter, r *http.Request) {
	principal, _ := transport.GetPrincipal(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message":   "Welcome Merchant! Bulk parcel shipping available.",
		"tenant_id": principal.TenantID,
		"role":      principal.Role,
	})
}

// handleAdminDashboard godoc
// @Summary      Admin dashboard status
// @Description  Test dashboard endpoint for authenticated Administrators.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} transport.ErrorEnvelope "Unauthorized"
// @Router       /api/v1/admin/dashboard [get]
func handleAdminDashboard(w http.ResponseWriter, r *http.Request) {
	principal, _ := transport.GetPrincipal(r.Context())
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Welcome Admin! System health: 100%.",
		"user_id": principal.UserID,
		"role":    principal.Role,
	})
}

// handleLegacyHealth godoc
// @Summary      System health status
// @Description  Returns general operational status and storage engine info.
// @Tags         system
// @Produce      json
// @Success      200 {object} map[string]string
// @Router       /api/v1/health [get]
func handleLegacyHealth(storageType string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"system":  "SwiftDrop Core API",
			"storage": storageType,
		})
	}
}

// handleHealthLive godoc
// @Summary      Kubernetes liveness probe
// @Description  Probes whether the Core API service process is alive.
// @Tags         system
// @Produce      json
// @Success      200 {object} map[string]string
// @Router       /health/live [get]
func handleHealthLive(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}

// handleHealthReady godoc
// @Summary      Kubernetes readiness probe
// @Description  Probes whether the Core API service is ready to accept traffic (verifies database connectivity).
// @Tags         system
// @Produce      json
// @Success      200 {object} map[string]string
// @Failure      503 {object} map[string]string "Database unreachable"
// @Router       /health/ready [get]
func handleHealthReady(dbConn *sql.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		if dbConn == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status":   "degraded",
				"database": "down",
			})
			return
		}

		pingCtx, pingCancel := context.WithTimeout(r.Context(), 1*time.Second)
		defer pingCancel()

		if err := dbConn.PingContext(pingCtx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]string{
				"status":   "degraded",
				"database": "down",
			})
			return
		}

		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":   "ok",
			"database": "connected",
		})
	}
}
