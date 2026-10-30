package domain

import (
	"time"
)

// RefreshTokenSession represents a persisted refresh token session for Single-Sign-On / Single-Use Rotation.
type RefreshTokenSession struct {
	ID        string
	UserID    string
	TokenHash string
	FamilyID  string
	IsRevoked bool
	ExpiresAt time.Time
	UsedAt    *time.Time
	CreatedAt time.Time
}

// NewRefreshTokenSession initializes a fresh refresh token session.
func NewRefreshTokenSession(
	id string,
	userID string,
	tokenHash string,
	familyID string,
	ttl time.Duration,
) *RefreshTokenSession {
	now := time.Now().UTC()
	return &RefreshTokenSession{
		ID:        id,
		UserID:    userID,
		TokenHash: tokenHash,
		FamilyID:  familyID,
		IsRevoked: false,
		ExpiresAt: now.Add(ttl),
		UsedAt:    nil,
		CreatedAt: now,
	}
}

// IsValid checks if the refresh token session is active, not revoked, and not expired.
func (s *RefreshTokenSession) IsValid() bool {
	if s.IsRevoked {
		return false
	}
	if s.UsedAt != nil {
		return false
	}
	return time.Now().UTC().Before(s.ExpiresAt)
}

// MarkUsed flags the session as used during token rotation.
func (s *RefreshTokenSession) MarkUsed() {
	now := time.Now().UTC()
	s.UsedAt = &now
}

// Revoke revokes the session explicitly.
func (s *RefreshTokenSession) Revoke() {
	s.IsRevoked = true
}
