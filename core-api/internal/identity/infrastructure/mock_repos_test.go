package infrastructure

import (
	"context"
	"testing"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
)

func TestMockUserRepository(t *testing.T) {
	ctx := context.Background()
	hasher := NewStandardPasswordHasher()
	repo := NewMockUserRepository(hasher)

	t.Run("pre-seeded 5 role accounts are queryable", func(t *testing.T) {
		users, total, err := repo.ListUsers(ctx, nil, 10, 0)
		if err != nil {
			t.Fatalf("ListUsers failed: %v", err)
		}
		if total != 5 || len(users) != 5 {
			t.Errorf("expected 5 seeded users, got total=%d len=%d", total, len(users))
		}

		admin, err := repo.FindByEmail(ctx, "admin@swiftdrop.local")
		if err != nil {
			t.Fatalf("FindByEmail failed: %v", err)
		}
		if admin.Role != domain.RoleAdmin {
			t.Errorf("expected ADMIN role, got %q", admin.Role)
		}

		byID, err := repo.FindByID(ctx, admin.ID)
		if err != nil {
			t.Fatalf("FindByID failed: %v", err)
		}
		if byID.Email.String() != "admin@swiftdrop.local" {
			t.Errorf("unexpected email %q", byID.Email.String())
		}
	})

	t.Run("Save new user and Update existing user", func(t *testing.T) {
		email, _ := domain.NewEmail("new.tester@swiftdrop.local")
		u := domain.NewUser("usr_new_1", email, "hash", "New Tester", "+94771112233", domain.RoleCustomer, nil, domain.StatusActive, time.Time{}, time.Time{})

		if err := repo.Save(ctx, u); err != nil {
			t.Fatalf("Save failed: %v", err)
		}

		u.DisplayName = "Updated Tester"
		if err := repo.Save(ctx, u); err != nil {
			t.Fatalf("Save update failed: %v", err)
		}

		fetched, err := repo.FindByID(ctx, "usr_new_1")
		if err != nil {
			t.Fatalf("FindByID failed: %v", err)
		}
		if fetched.DisplayName != "Updated Tester" {
			t.Errorf("expected Updated Tester, got %q", fetched.DisplayName)
		}
	})

	t.Run("ListUsers filters by role and paginates", func(t *testing.T) {
		courierRole := domain.RoleCourier
		couriers, total, err := repo.ListUsers(ctx, &courierRole, 10, 0)
		if err != nil {
			t.Fatalf("ListUsers failed: %v", err)
		}
		if total != 1 || len(couriers) != 1 || couriers[0].Role != domain.RoleCourier {
			t.Errorf("expected 1 courier, got total=%d len=%d", total, len(couriers))
		}

		paged, _, err := repo.ListUsers(ctx, nil, 2, 1)
		if err != nil {
			t.Fatalf("ListUsers pagination failed: %v", err)
		}
		if len(paged) != 2 {
			t.Errorf("expected 2 users on page, got %d", len(paged))
		}
	})

	t.Run("not found errors", func(t *testing.T) {
		if _, err := repo.FindByEmail(ctx, "missing@swiftdrop.local"); err != domain.ErrUserNotFound {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
		if _, err := repo.FindByID(ctx, "usr_nonexistent"); err != domain.ErrUserNotFound {
			t.Errorf("expected ErrUserNotFound, got %v", err)
		}
	})
}

func TestMockTokenRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMockTokenRepository()

	s1 := domain.NewRefreshTokenSession("tok_1", "usr_1", "hash_1", "fam_A", 10*time.Minute)
	s2 := domain.NewRefreshTokenSession("tok_2", "usr_1", "hash_2", "fam_A", 10*time.Minute)

	if err := repo.SaveToken(ctx, s1); err != nil {
		t.Fatalf("SaveToken s1 failed: %v", err)
	}
	if err := repo.SaveToken(ctx, s2); err != nil {
		t.Fatalf("SaveToken s2 failed: %v", err)
	}

	got, err := repo.FindByTokenHash(ctx, "hash_1")
	if err != nil || got.ID != "tok_1" {
		t.Fatalf("FindByTokenHash failed: got=%+v err=%v", got, err)
	}

	if _, err := repo.FindByTokenHash(ctx, "unknown_hash"); err != domain.ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired for missing hash, got %v", err)
	}

	if err := repo.MarkTokenUsed(ctx, "tok_1"); err != nil {
		t.Fatalf("MarkTokenUsed failed: %v", err)
	}
	got1, _ := repo.FindByTokenHash(ctx, "hash_1")
	if got1.UsedAt == nil {
		t.Errorf("expected UsedAt to be set")
	}

	if err := repo.MarkTokenUsed(ctx, "missing_id"); err != domain.ErrTokenExpired {
		t.Errorf("expected ErrTokenExpired for missing token id, got %v", err)
	}

	if err := repo.RevokeTokenFamily(ctx, "fam_A"); err != nil {
		t.Fatalf("RevokeTokenFamily failed: %v", err)
	}
	got2, _ := repo.FindByTokenHash(ctx, "hash_2")
	if !got2.IsRevoked {
		t.Errorf("expected tok_2 to be revoked with family")
	}
}

func TestMockOAuthRepository(t *testing.T) {
	ctx := context.Background()
	repo := NewMockOAuthRepository()

	link := domain.NewOAuthIdentity("oa_1", "usr_1", domain.ProviderGoogle, "sub_1", "alice@gmail.com")
	if err := repo.SaveLink(ctx, link); err != nil {
		t.Fatalf("SaveLink failed: %v", err)
	}

	got, err := repo.FindByProviderSubject(ctx, domain.ProviderGoogle, "sub_1")
	if err != nil || got.UserID != "usr_1" {
		t.Fatalf("FindByProviderSubject failed: got=%+v err=%v", got, err)
	}

	if _, err := repo.FindByProviderSubject(ctx, domain.ProviderGoogle, "missing"); err == nil {
		t.Errorf("expected error for missing provider subject")
	}

	// Update existing link
	updated := domain.NewOAuthIdentity("oa_1", "usr_1", domain.ProviderGoogle, "sub_1", "alice.new@gmail.com")
	_ = repo.SaveLink(ctx, updated)

	byUser, err := repo.FindByUserID(ctx, "usr_1")
	if err != nil {
		t.Fatalf("FindByUserID failed: %v", err)
	}
	if len(byUser) != 1 || byUser[0].Email != "alice.new@gmail.com" {
		t.Errorf("expected 1 updated link, got %+v", byUser)
	}
}

func TestMockAuditWriterAndDevEmailSender(t *testing.T) {
	ctx := context.Background()

	writer := NewMockAuditWriter()
	entry := application.AuditEntry{
		ActorID: "usr_1",
		Action:  "USER_LOGIN",
	}
	if err := writer.WriteAuditLog(ctx, entry); err != nil {
		t.Fatalf("WriteAuditLog failed: %v", err)
	}
	if len(writer.Entries) != 1 || writer.Entries[0].Action != "USER_LOGIN" {
		t.Errorf("unexpected audit entries: %+v", writer.Entries)
	}

	sender := NewDevEmailSender()
	if err := sender.SendOTPEmail(ctx, "test@swiftdrop.local", "123456"); err != nil {
		t.Errorf("SendOTPEmail failed: %v", err)
	}
	if err := sender.SendPasswordResetEmail(ctx, "test@swiftdrop.local", "654321"); err != nil {
		t.Errorf("SendPasswordResetEmail failed: %v", err)
	}
}
