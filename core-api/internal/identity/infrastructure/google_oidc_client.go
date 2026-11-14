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

// GoogleOIDCClient validates Google OAuth ID tokens against Google's OIDC tokeninfo endpoint.
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
	if strings.TrimSpace(c.clientID) == "" {
		return nil, fmt.Errorf("%w: GOOGLE_CLIENT_ID is not configured", domain.ErrInvalidToken)
	}

	// Online verification via Google's tokeninfo endpoint
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

	emailVerified := info.EmailVerified == "true" || info.EmailVerified == "1"
	if !emailVerified {
		return nil, fmt.Errorf("%w: google account email is not verified", domain.ErrInvalidToken)
	}

	return &application.OIDCClaims{
		Subject:       info.Sub,
		Email:         strings.ToLower(info.Email),
		EmailVerified: true,
		Name:          info.Name,
		GivenName:     info.GivenName,
		FamilyName:    info.FamilyName,
		Picture:       info.Picture,
	}, nil
}
