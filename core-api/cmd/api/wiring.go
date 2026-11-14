package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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
// It prioritizes PostgreSQL if running, automatically applies migrations,
// and gracefully falls back to mock storage if PostgreSQL is not yet started.
func BuildAppContainer() *AppContainer {
	config.LoadEnv(".env", "../.env", "../../.env")

	// 1. Password Hasher & JWT Token Provider
	hasher := infrastructure.NewStandardPasswordHasher()
	jwtSecret, err := resolveJWTSecret()
	if err != nil {
		log.Fatalf("[FATAL] %v", err)
	}
	tokenProvider := infrastructure.NewJWTTokenProvider(jwtSecret)

	var userRepo application.UserRepository
	var tokenRepo application.TokenRepository
	var auditWriter application.AuditWriter
	var oauthRepo application.OAuthRepository
	var otpRepo application.OTPRepository
	var dbConn *sql.DB
	storageType := "In-Memory (Mock Role Accounts)"

	// 2. Attempt PostgreSQL Connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://swiftdrop:swiftdrop_dev@localhost:5432/swiftdrop_db?sslmode=disable"
	}

	db, err := sql.Open("postgres", dbURL)
	if err == nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		if pingErr := db.PingContext(ctx); pingErr == nil {
			log.Println("[INFO] Successfully connected to PostgreSQL database.")

			// Auto-run migrations & seed mock accounts in PostgreSQL
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

			// Production: the demo accounts from migration 20261021000002 have published
			// passwords. Rotate them (SEED_PASSWORD_<ROLE>) or suspend them.
			if config.IsProduction() {
				report, seedErr := application.SecureSeedAccounts(context.Background(), userRepo, hasher, seedPasswordsFromEnv())
				if seedErr != nil {
					log.Fatalf("[FATAL] Could not secure demo accounts: %v", seedErr)
				}
				log.Printf("[INFO] Demo accounts secured: rotated=%v suspended=%v unchanged=%v missing=%v\n",
					report.Rotated, report.Suspended, report.Unchanged, report.Missing)
			}
		} else {
			if config.IsProduction() {
				// Never serve the in-memory demo accounts in production.
				log.Fatalf("[FATAL] PostgreSQL is not reachable (%v); refusing to start in production", pingErr)
			}
			log.Printf("[NOTICE] PostgreSQL is not running at %s.\n", dbURL)
			log.Println("[NOTICE] Using pre-seeded mock accounts so you and your team can develop immediately!")
		}
	} else if config.IsProduction() {
		log.Fatalf("[FATAL] Invalid DATABASE_URL; refusing to start in production: %v", err)
	}

	// 3. Fallback to Pre-Seeded Mock Repository if PostgreSQL is offline
	if userRepo == nil {
		mockRepo := infrastructure.NewMockUserRepository(hasher)
		// Pre-seed user's email so password recovery OTP can be dispatched immediately
		devEmails := []string{"kgumigiranga@gmail.com"}
		for i, devEmail := range devEmails {
			if emailVO, err := domain.NewEmail(devEmail); err == nil {
				pwdHash, _ := hasher.HashPassword("CustomerPassword123!")
				now := time.Now().UTC()
				devUser := domain.NewUser(
					fmt.Sprintf("usr_01HKGUMIGIRANGA%014d", i+1),
					emailVO,
					pwdHash,
					"Umesh Mihiranga",
					"+94770000006",
					domain.RoleCustomer,
					nil,
					domain.StatusActive,
					now,
					now,
				)
				_ = mockRepo.Save(context.Background(), devUser)
			}
		}
		userRepo = mockRepo
		tokenRepo = infrastructure.NewMockTokenRepository()
		auditWriter = infrastructure.NewMockAuditWriter()
		oauthRepo = infrastructure.NewMockOAuthRepository()
	}

	// 4. Application Services
	if otpRepo == nil {
		otpRepo = infrastructure.NewMemoryOTPRepository()
	}

	// Email Sender Adapter:
	// - If SMTP_HOST is configured in environment variables, sends real emails via SMTPEmailSender.
	// - Otherwise, falls back to DevEmailSender (prints 6-digit OTP to server console for local testing).
	var emailSender application.EmailSender
	smtpHost := strings.TrimSpace(os.Getenv("SMTP_HOST"))
	if smtpHost != "" {
		smtpPort := strings.TrimSpace(os.Getenv("SMTP_PORT"))
		if smtpPort == "" {
			smtpPort = "587"
		}
		smtpUser := strings.TrimSpace(os.Getenv("SMTP_USER"))
		if smtpUser == "" {
			smtpUser = strings.TrimSpace(os.Getenv("SMTP_USERNAME"))
		}
		smtpPass := strings.TrimSpace(os.Getenv("SMTP_PASS"))
		if smtpPass == "" {
			smtpPass = strings.TrimSpace(os.Getenv("SMTP_PASSWORD"))
		}
		smtpFromEmail := strings.TrimSpace(os.Getenv("SMTP_FROM_EMAIL"))
		if smtpFromEmail == "" {
			smtpFromEmail = strings.TrimSpace(os.Getenv("SMTP_FROM"))
		}
		if smtpFromEmail == "" || (strings.Contains(smtpHost, "gmail") && smtpUser != "" && strings.Contains(smtpUser, "@")) {
			if strings.Contains(smtpHost, "gmail") && smtpUser != "" {
				smtpFromEmail = smtpUser
			} else if smtpFromEmail == "" {
				smtpFromEmail = "no-reply@swiftdrop.com"
			}
		}
		smtpFromName := strings.TrimSpace(os.Getenv("SMTP_FROM_NAME"))
		if smtpFromName == "" {
			smtpFromName = "SwiftDrop Deliveries"
		}

		emailSender = infrastructure.NewSMTPEmailSender(smtpHost, smtpPort, smtpUser, smtpPass, smtpFromEmail, smtpFromName)
		log.Printf("[INFO] Using SMTP Email Sender (%s:%s, from: %s <%s>, user: %s)\n", smtpHost, smtpPort, smtpFromName, smtpFromEmail, smtpUser)
	} else {
		emailSender = infrastructure.NewDevEmailSender()
		log.Println("[INFO] Using Dev Email Sender (OTP printed to console). Set SMTP_HOST in .env to send real emails.")
	}

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

	// 10. Register Member 02 Pricing & Parcel Operations
	if dbConn != nil {
		if err := registerPricingModule(mux, dbConn, security); err != nil {
			if config.IsProduction() {
				log.Fatalf("[FATAL] Failed to initialize Member 02 Pricing: %v", err)
			}
			log.Printf("[NOTICE] Member 02 Pricing not fully active in local mode: %v\n", err)
		}
		if err := registerParcelsModule(mux, dbConn, security); err != nil {
			if config.IsProduction() {
				log.Fatalf("[FATAL] Failed to initialize Member 02 Parcels: %v", err)
			}
			log.Printf("[NOTICE] Member 02 Parcels not fully active in local mode: %v\n", err)
		}
	} else {
		log.Println("[NOTICE] Member 02 Pricing & Parcels not fully active: PostgreSQL connection (*sql.DB) is required")
	}

	// 11. Register Member 03 Dispatch & Delivery Modules
	registerDispatchAndDeliveryModules(mux, dbConn, security)

	// 12. Register Member 07 Admin Operations (zones, postal codes, settings, audit log, delivery report)
	if dbConn != nil {
		if err := registerAdminModule(mux, dbConn, security); err != nil {
			if config.IsProduction() {
				log.Fatalf("[FATAL] Failed to initialize Member 07 Admin Operations: %v", err)
			}
			log.Printf("[NOTICE] Member 07 Admin Operations not active in local mode: %v\n", err)
		}
	} else {
		log.Println("[NOTICE] Member 07 Admin Operations not active: PostgreSQL connection (*sql.DB) is required")
	}

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
// @Description  Test portal endpoint for merchant users. Requires a tenant in the token: platform Admins have no tenant and get 403 AUTH_TENANT_REQUIRED.
// @Tags         admin
// @Produce      json
// @Security     BearerAuth
// @Success      200 {object} map[string]interface{}
// @Failure      401 {object} transport.ErrorEnvelope "Unauthorized"
// @Failure      403 {object} transport.ErrorEnvelope "Wrong role, or no tenant (AUTH_TENANT_REQUIRED)"
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
		appEnv := strings.ToLower(strings.TrimSpace(os.Getenv("APP_ENV")))
		if appEnv == "" {
			appEnv = "local"
		}

		if dbConn == nil {
			if appEnv == "local" {
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(map[string]string{
					"status":   "ok",
					"database": "mock",
				})
				return
			}
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
