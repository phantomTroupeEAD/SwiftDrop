package infrastructure

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/swiftdrop/core-api/internal/identity/application"
	"github.com/swiftdrop/core-api/internal/identity/domain"
)

// GoogleOIDCClient validates Google OAuth ID tokens.
// In dev/test mode (or when given tokens prefixed with "mock_" or "dev_"),
// it produces deterministic mock claims without external network requests.
type GoogleOIDCClient struct {
	clientID   string
	httpClient *http.Client
}

// NewGoogleOIDCClient constructs a Google OIDC verifier.
func NewGoogleOIDCClient(clientID string) *GoogleOIDCClient {
	return &GoogleOIDCClient{
		clientID: clientID,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

type googleTokenInfoResponse struct {
	Sub           string `json:"sub"`
	Email         string `json:"email"`
	EmailVerified string `json:"email_verified"` // Google returns "true" as string or boolean in some endpoints
	Name          string `json:"name"`
	GivenName     string `json:"given_name"`
	FamilyName    string `json:"family_name"`
	Picture       string `json:"picture"`
	Aud           string `json:"aud"`
	Error         string `json:"error_description"`
}

// VerifyIDToken validates a Google ID token and extracts claims.
func (c *GoogleOIDCClient) VerifyIDToken(ctx context.Context, idToken string) (*application.OIDCClaims, error) {
	trimmed := strings.TrimSpace(idToken)
	if trimmed == "" {
		return nil, domain.ErrInvalidToken
	}

	// 1. Check for Mock/Offline Dev Tokens
	if strings.HasPrefix(trimmed, "mock_") || strings.HasPrefix(trimmed, "dev_") || c.clientID == "" {
		return c.resolveMockToken(trimmed)
	}

	// 2. Online verification via Google's tokeninfo endpoint
	endpoint := fmt.Sprintf("https://oauth2.googleapis.com/tokeninfo?id_token=%s", url.QueryEscape(trimmed))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create tokeninfo request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to contact google tokeninfo: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("%w: google token rejected (%s)", domain.ErrInvalidToken, strings.TrimSpace(string(body)))
	}

	var info googleTokenInfoResponse
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("failed to parse google token response: %w", err)
	}

	if info.Sub == "" || info.Email == "" {
		return nil, domain.ErrInvalidToken
	}

	// Verify audience if configured
	if c.clientID != "" && info.Aud != c.clientID {
		return nil, fmt.Errorf("%w: token audience mismatch", domain.ErrInvalidToken)
	}

	return &application.OIDCClaims{
		Subject:       info.Sub,
		Email:         strings.ToLower(info.Email),
		EmailVerified: info.EmailVerified == "true" || info.EmailVerified == "1",
		Name:          info.Name,
		GivenName:     info.GivenName,
		FamilyName:    info.FamilyName,
		Picture:       info.Picture,
	}, nil
}

// resolveMockToken provides simulated claims for automated tests and offline development.
func (c *GoogleOIDCClient) resolveMockToken(token string) (*application.OIDCClaims, error) {
	if token == "mock_invalid_token" {
		return nil, domain.ErrInvalidToken
	}

	// Default fallback values
	email := "google.user@example.com"
	sub := "google_sub_1001"
	name := "Google TestUser"
	givenName := "Google"
	familyName := "TestUser"

	// Parse identifiers if provided, e.g. "mock_google_id_token_alice" or "mock_alice@gmail.com"
	clean := strings.TrimPrefix(token, "mock_")
	clean = strings.TrimPrefix(clean, "dev_")
	clean = strings.TrimPrefix(clean, "google_id_token_")

	if strings.Contains(clean, "@") {
		email = clean
		parts := strings.Split(clean, "@")
		sub = "google_sub_" + parts[0]
		name = strings.Title(parts[0]) + " OAuth"
		givenName = strings.Title(parts[0])
		familyName = "OAuth"
	} else if clean != "" {
		email = clean + "@gmail.com"
		sub = "google_sub_" + clean
		name = strings.Title(clean) + " User"
		givenName = strings.Title(clean)
		familyName = "User"
	}

	return &application.OIDCClaims{
		Subject:       sub,
		Email:         strings.ToLower(email),
		EmailVerified: true,
		Name:          name,
		GivenName:     givenName,
		FamilyName:    familyName,
		Picture:       "https://lh3.googleusercontent.com/a/mock-avatar",
	}, nil
}
