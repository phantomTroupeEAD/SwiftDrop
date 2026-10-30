package infrastructure

import (
	"context"
	"sync"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// MockTokenRepository is an in-memory refresh token store.
type MockTokenRepository struct {
	mu     sync.RWMutex
	tokens map[string]*domain.RefreshTokenSession // keyed by tokenHash
}

// NewMockTokenRepository constructs a new in-memory token repo.
func NewMockTokenRepository() *MockTokenRepository {
	return &MockTokenRepository{
		tokens: make(map[string]*domain.RefreshTokenSession),
	}
}

// SaveToken persists a new session.
func (r *MockTokenRepository) SaveToken(_ context.Context, session *domain.RefreshTokenSession) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	clone := *session
	r.tokens[session.TokenHash] = &clone
	return nil
}

// FindByTokenHash looks up a session by token hash.
func (r *MockTokenRepository) FindByTokenHash(_ context.Context, tokenHash string) (*domain.RefreshTokenSession, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	s, exists := r.tokens[tokenHash]
	if !exists {
		return nil, domain.ErrTokenExpired
	}

	clone := *s
	return &clone, nil
}

// MarkTokenUsed marks a token as used for rotation.
func (r *MockTokenRepository) MarkTokenUsed(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, s := range r.tokens {
		if s.ID == id {
			s.MarkUsed()
			return nil
		}
	}
	return domain.ErrTokenExpired
}

// RevokeTokenFamily invalidates all tokens in a family if reuse is detected.
func (r *MockTokenRepository) RevokeTokenFamily(_ context.Context, familyID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, s := range r.tokens {
		if s.FamilyID == familyID {
			s.Revoke()
		}
	}
	return nil
}
