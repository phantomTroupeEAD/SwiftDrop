package domain

import (
	"testing"
	"time"
)

func TestRefreshTokenSession(t *testing.T) {
	t.Run("fresh session is valid", func(t *testing.T) {
		s := NewRefreshTokenSession("tok_1", "usr_1", "hash_1", "fam_1", 15*time.Minute)
		if !s.IsValid() {
			t.Errorf("expected fresh session to be valid")
		}
		if s.IsRevoked || s.UsedAt != nil {
			t.Errorf("expected fresh session not to be revoked or used")
		}
	})

	t.Run("marked used session is invalid", func(t *testing.T) {
		s := NewRefreshTokenSession("tok_2", "usr_1", "hash_2", "fam_1", 15*time.Minute)
		s.MarkUsed()
		if s.UsedAt == nil {
			t.Fatalf("expected UsedAt to be populated")
		}
		if s.IsValid() {
			t.Errorf("expected used session to be invalid")
		}
	})

	t.Run("revoked session is invalid", func(t *testing.T) {
		s := NewRefreshTokenSession("tok_3", "usr_1", "hash_3", "fam_1", 15*time.Minute)
		s.Revoke()
		if !s.IsRevoked {
			t.Fatalf("expected IsRevoked to be true")
		}
		if s.IsValid() {
			t.Errorf("expected revoked session to be invalid")
		}
	})

	t.Run("expired session is invalid", func(t *testing.T) {
		s := NewRefreshTokenSession("tok_4", "usr_1", "hash_4", "fam_1", -1*time.Minute)
		if s.IsValid() {
			t.Errorf("expected expired session to be invalid")
		}
	})
}

func TestNewOAuthIdentity(t *testing.T) {
	id := NewOAuthIdentity("oauth_1", "usr_1", ProviderGoogle, "sub_123", "alice@gmail.com")
	if id.ID != "oauth_1" || id.UserID != "usr_1" || id.Provider != ProviderGoogle || id.ProviderSubject != "sub_123" || id.Email != "alice@gmail.com" {
		t.Errorf("unexpected OAuthIdentity fields: %+v", id)
	}
	if id.CreatedAt.IsZero() {
		t.Errorf("expected non-zero CreatedAt")
	}
}
