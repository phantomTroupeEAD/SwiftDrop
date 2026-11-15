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

func TestGoogleOIDCClient_OnlineVerification(t *testing.T) {
	ctx := context.Background()

	t.Run("rejects empty token", func(t *testing.T) {
		client := NewGoogleOIDCClient("my-client-id")
		if _, err := client.VerifyIDToken(ctx, "   "); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

	t.Run("rejects when GOOGLE_CLIENT_ID is unconfigured", func(t *testing.T) {
		client := NewGoogleOIDCClient("")
		if _, err := client.VerifyIDToken(ctx, "some-token"); !errors.Is(err, domain.ErrInvalidToken) {
			t.Errorf("expected ErrInvalidToken, got %v", err)
		}
	})

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
