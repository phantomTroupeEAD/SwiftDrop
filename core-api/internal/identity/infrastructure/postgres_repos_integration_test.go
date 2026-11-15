package infrastructure

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/swiftdrop/core-api/internal/identity/domain"
	"github.com/swiftdrop/core-api/migrations"
)

func openTestPostgresDB(t *testing.T) *sql.DB {
	t.Helper()

	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set; skipping PostgreSQL integration test")
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		t.Fatalf("failed to open postgres connection: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		t.Skipf("postgres ping failed (%v); skipping integration test", err)
	}

	if err := migrations.RunMigrations(ctx, db); err != nil {
		t.Fatalf("failed to run migrations: %v", err)
	}

	return db
}

func TestPostgresIdentityRepositories_Integration(t *testing.T) {
	db := openTestPostgresDB(t)
	ctx := context.Background()

	userRepo := NewPostgresUserRepository(db)
	tokenRepo := NewPostgresTokenRepository(db)
	oauthRepo := NewPostgresOAuthRepository(db)

	userID := "11111111-1111-1111-1111-111111111199"
	emailStr := "pg.integration." + time.Now().UTC().Format("20060102150405") + "@swiftdrop.local"
	email, err := domain.NewEmail(emailStr)
	if err != nil {
		t.Fatalf("NewEmail failed: %v", err)
	}

	// Clean up test user on exit
	t.Cleanup(func() {
		_, _ = db.ExecContext(context.Background(), "DELETE FROM oauth_identities WHERE user_id = $1", userID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM refresh_tokens WHERE user_id = $1", userID)
		_, _ = db.ExecContext(context.Background(), "DELETE FROM users WHERE id = $1", userID)
	})

	u := domain.NewUser(userID, email, "$sd$sha256$abcd$ef01", "PG Test User", "+94779998877", domain.RoleCustomer, nil, domain.StatusActive, time.Time{}, time.Time{})
	if err := userRepo.Save(ctx, u); err != nil {
		t.Fatalf("PostgresUserRepository.Save failed: %v", err)
	}

	byEmail, err := userRepo.FindByEmail(ctx, emailStr)
	if err != nil || byEmail.ID != userID {
		t.Fatalf("FindByEmail failed: got=%+v err=%v", byEmail, err)
	}

	byID, err := userRepo.FindByID(ctx, userID)
	if err != nil || byID.DisplayName != "PG Test User" {
		t.Fatalf("FindByID failed: got=%+v err=%v", byID, err)
	}

	// Token lifecycle
	tokID := "22222222-2222-2222-2222-222222222299"
	famID := "33333333-3333-3333-3333-333333333399"
	sess := domain.NewRefreshTokenSession(tokID, userID, "pg_hash_test_99", famID, 10*time.Minute)
	if err := tokenRepo.SaveToken(ctx, sess); err != nil {
		t.Fatalf("SaveToken failed: %v", err)
	}

	foundSess, err := tokenRepo.FindByTokenHash(ctx, "pg_hash_test_99")
	if err != nil || foundSess.ID != tokID {
		t.Fatalf("FindByTokenHash failed: got=%+v err=%v", foundSess, err)
	}

	if err := tokenRepo.MarkTokenUsed(ctx, tokID); err != nil {
		t.Fatalf("MarkTokenUsed failed: %v", err)
	}
	if err := tokenRepo.RevokeTokenFamily(ctx, famID); err != nil {
		t.Fatalf("RevokeTokenFamily failed: %v", err)
	}

	// OAuth lifecycle
	link := domain.NewOAuthIdentity("44444444-4444-4444-4444-444444444499", userID, domain.ProviderGoogle, "pg_sub_99", emailStr)
	if err := oauthRepo.SaveLink(ctx, link); err != nil {
		t.Fatalf("SaveLink failed: %v", err)
	}
	foundLink, err := oauthRepo.FindByProviderSubject(ctx, domain.ProviderGoogle, "pg_sub_99")
	if err != nil || foundLink.UserID != userID {
		t.Fatalf("FindByProviderSubject failed: got=%+v err=%v", foundLink, err)
	}
}
