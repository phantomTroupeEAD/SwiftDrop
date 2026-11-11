package domain

import "time"

// Provider constants for supported federated identity providers.
const (
	ProviderGoogle = "GOOGLE"
)

// OAuthIdentity represents a federated identity link connecting a third-party
// account (e.g. Google) to an internal SwiftDrop User aggregate.
type OAuthIdentity struct {
	ID              string
	UserID          string
	Provider        string
	ProviderSubject string
	Email           string
	CreatedAt       time.Time
}

// NewOAuthIdentity constructs an OAuthIdentity aggregate.
func NewOAuthIdentity(id, userID, provider, subject, email string) *OAuthIdentity {
	return &OAuthIdentity{
		ID:              id,
		UserID:          userID,
		Provider:        provider,
		ProviderSubject: subject,
		Email:           email,
		CreatedAt:       time.Now().UTC(),
	}
}
