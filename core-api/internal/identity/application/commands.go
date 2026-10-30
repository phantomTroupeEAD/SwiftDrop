package application

import (
	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// LoginCommand encapsulates the parameters for user authentication.
type LoginCommand struct {
	Email     string
	Password  string
	ClientIP  string
	UserAgent string
}

// LoginResult contains the authenticated access token, metadata, and user info.
type LoginResult struct {
	AccessToken  string
	TokenType    string
	ExpiresIn    int // in seconds, typically 900 (15 min)
	RefreshToken string
	User         *domain.User
}

// RefreshSessionCommand parameters for token rotation.
type RefreshSessionCommand struct {
	RawRefreshToken string
	ClientIP        string
	UserAgent       string
}
