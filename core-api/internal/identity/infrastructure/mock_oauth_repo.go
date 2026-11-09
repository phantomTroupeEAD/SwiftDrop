package infrastructure

import (
	"context"
	"fmt"
	"sync"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// MockOAuthRepository is a thread-safe in-memory store for federated OAuth identity links.
type MockOAuthRepository struct {
	mu            sync.RWMutex
	linksByKey    map[string]*domain.OAuthIdentity   // key: "PROVIDER:SUBJECT"
	linksByUserID map[string][]*domain.OAuthIdentity // key: UserID
}

// NewMockOAuthRepository creates an empty in-memory OAuth repository.
func NewMockOAuthRepository() *MockOAuthRepository {
	return &MockOAuthRepository{
		linksByKey:    make(map[string]*domain.OAuthIdentity),
		linksByUserID: make(map[string][]*domain.OAuthIdentity),
	}
}

func makeLinkKey(provider, subject string) string {
	return fmt.Sprintf("%s:%s", provider, subject)
}

// SaveLink stores or updates an OAuth link.
func (r *MockOAuthRepository) SaveLink(ctx context.Context, identity *domain.OAuthIdentity) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := makeLinkKey(identity.Provider, identity.ProviderSubject)
	r.linksByKey[key] = identity

	// Update list by user ID
	existingList := r.linksByUserID[identity.UserID]
	updated := false
	for i, link := range existingList {
		if link.Provider == identity.Provider && link.ProviderSubject == identity.ProviderSubject {
			existingList[i] = identity
			updated = true
			break
		}
	}
	if !updated {
		r.linksByUserID[identity.UserID] = append(existingList, identity)
	}

	return nil
}

// FindByProviderSubject finds an identity link by provider and provider subject ID.
func (r *MockOAuthRepository) FindByProviderSubject(ctx context.Context, provider, subject string) (*domain.OAuthIdentity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeLinkKey(provider, subject)
	link, exists := r.linksByKey[key]
	if !exists {
		return nil, fmt.Errorf("oauth link not found")
	}

	return link, nil
}

// FindByUserID retrieves all OAuth links associated with a user.
func (r *MockOAuthRepository) FindByUserID(ctx context.Context, userID string) ([]*domain.OAuthIdentity, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	list := r.linksByUserID[userID]
	result := make([]*domain.OAuthIdentity, len(list))
	copy(result, list)
	return result, nil
}
