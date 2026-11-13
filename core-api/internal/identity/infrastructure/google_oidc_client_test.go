package infrastructure

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/swiftdrop/core-api/internal/identity/domain"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestGoogleOIDCClient_MockTokens(t *testing.T) {
	ctx := context.Background()
	client := NewGoogleOIDCClient("")

	t.Run("rejects empty token", func(t *testing.T) {
		if _, err := client.VerifyIDToken(ctx, "   "); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("rejects mock_invalid_token", func(t *testing.T) {
		if _, err := client.VerifyIDToken(ctx, "mock_invalid_token"); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("parses mock email token", func(t *testing.T) {
		claims, err := client.VerifyIDToken(ctx, "mock_alice@gmail.com")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if claims.Email != "alice@gmail.com" || claims.Subject != "google_sub_alice" || !claims.EmailVerified {
			t.Errorf("unexpected claims: %+v", claims)
		}
	})

	t.Run("parses mock_google_id_token_bob", func(t *testing.T) {
		claims, err := client.VerifyIDToken(ctx, "mock_google_id_token_bob")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if claims.Email != "bob@gmail.com" || claims.Subject != "google_sub_bob" {
			t.Errorf("unexpected claims: %+v", claims)
		}
	})
}

func TestGoogleOIDCClient_OnlineVerification(t *testing.T) {
	ctx := context.Background()

	t.Run("validates tokeninfo 200 response and audience", func(t *testing.T) {
		client := NewGoogleOIDCClient("my-google-client-id")
		client.httpClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				body := `{"sub":"12345","email":"User@Gmail.com","email_verified":"true","name":"Test User","given_name":"Test","family_name":"User","aud":"my-google-client-id"}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		}

		claims, err := client.VerifyIDToken(ctx, "real-id-token-jwt")
		if err != nil {
			t.Fatalf("expected verification to succeed, got %v", err)
		}
		if claims.Email != "user@gmail.com" || claims.Subject != "12345" || !claims.EmailVerified {
			t.Errorf("unexpected claims: %+v", claims)
		}
	})

	t.Run("rejects audience mismatch", func(t *testing.T) {
		client := NewGoogleOIDCClient("expected-client-id")
		client.httpClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				body := `{"sub":"12345","email":"user@gmail.com","aud":"wrong-client-id"}`
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       io.NopCloser(strings.NewReader(body)),
				}, nil
			}),
		}

		if _, err := client.VerifyIDToken(ctx, "real-id-token-jwt"); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken for audience mismatch, got %v", err)
		}
	})

	t.Run("rejects non-200 tokeninfo response", func(t *testing.T) {
		client := NewGoogleOIDCClient("my-client-id")
		client.httpClient = &http.Client{
			Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode: http.StatusBadRequest,
					Body:       io.NopCloser(strings.NewReader(`{"error_description":"Invalid Value"}`)),
				}, nil
			}),
		}

		if _, err := client.VerifyIDToken(ctx, "bad-token"); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})
}
