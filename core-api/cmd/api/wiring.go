package main

import (
	"context"
	"database/sql"
	"encoding/json"
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
	// 1. Password Hasher & JWT Token Provider
	hasher := infrastructure.NewStandardPasswordHasher()
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		// #nosec G101 -- Local development fallback key
		jwtSecret = "swiftdrop-development-secret-key-change-in-production-32chars" //nolint:gosec
	}
	tokenProvider := infrastructure.NewJWTTokenProvider(jwtSecret)

	var userRepo application.UserRepository
	var tokenRepo application.TokenRepository
	var auditWriter application.AuditWriter
	var oauthRepo application.OAuthRepository
	var dbConn *sql.DB
	storageType := "In-Memory (Mock Role Accounts)"

	// 2. Attempt PostgreSQL Connection
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		dbURL = "postgres://postgres:postgres@localhost:5432/swiftdrop?sslmode=disable"
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
			dbConn = db
			storageType = "PostgreSQL Database (Authoritative)"
		} else {
			log.Printf("[NOTICE] PostgreSQL is not running at %s.\n", dbURL)
			log.Println("[NOTICE] Using pre-seeded mock accounts so you and your team can develop immediately!")
		}
	}

	// 3. Fallback to Pre-Seeded Mock Repository if PostgreSQL is offline
	if userRepo == nil {
		userRepo = infrastructure.NewMockUserRepository(hasher)
		tokenRepo = infrastructure.NewMockTokenRepository()
		auditWriter = infrastructure.NewMockAuditWriter()
		oauthRepo = infrastructure.NewMockOAuthRepository()
	}

	// 4. Application Services
	otpRepo := infrastructure.NewMemoryOTPRepository()

	// Email Sender Adapter:
	// - If SMTP_HOST is configured in environment variables, sends real emails via SMTPEmailSender.
	// - Otherwise, falls back to DevEmailSender (prints 6-digit OTP to server console for local testing).
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
		log.Printf("[INFO] Using production SMTP Email Sender (%s:%s)\n", smtpHost, smtpPort)
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
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				principal, _ := transport.GetPrincipal(r.Context())
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"message":   "Welcome Courier! Your courier routes are active.",
					"user_id":   principal.UserID,
					"role":      principal.Role,
					"active_id": "route_colombo_central",
				})
			}),
		),
	))

	mux.Handle("GET /api/v1/merchant/portal", security.Authenticate(
		transport.RequireRole(domain.RoleMerchant, domain.RoleAdmin)(
			transport.RequireTenant()(
				http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					principal, _ := transport.GetPrincipal(r.Context())
					w.Header().Set("Content-Type", "application/json")
					_ = json.NewEncoder(w).Encode(map[string]interface{}{
						"message":   "Welcome Merchant! Bulk parcel shipping available.",
						"tenant_id": principal.TenantID,
						"role":      principal.Role,
					})
				}),
			),
		),
	))

	mux.Handle("GET /api/v1/admin/dashboard", security.Authenticate(
		transport.RequireRole(domain.RoleAdmin)(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				principal, _ := transport.GetPrincipal(r.Context())
				w.Header().Set("Content-Type", "application/json")
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"message": "Welcome Admin! System health: 100%.",
					"user_id": principal.UserID,
					"role":    principal.Role,
				})
			}),
		),
	))

	// Legacy Health Endpoint
	mux.HandleFunc("GET /api/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status":  "healthy",
			"system":  "SwiftDrop Core API",
			"storage": storageType,
		})
	})

	// Standard Kubernetes / Infrastructure Health Probes (Phase 8B)
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"status": "ok",
		})
	})

	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
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
	})

	// 8. Register Member 08 Events & Messaging Operations & Swagger UI
	registerEventsModules(mux, dbConn, security)

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




