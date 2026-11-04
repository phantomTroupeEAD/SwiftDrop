package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	identityApp "github.com/swiftdrop/core-api/internal/identity/application"
	identityDomain "github.com/swiftdrop/core-api/internal/identity/domain"
)

func TestDispatchAndDeliveryWiring_RoutesRegistered(t *testing.T) {
	origEnv := os.Getenv("APP_ENV")
	defer os.Setenv("APP_ENV", origEnv)
	_ = os.Setenv("APP_ENV", "local")
	_ = os.Setenv("DATABASE_URL", "postgres://none:none@127.0.0.1:54399/none?sslmode=disable")

	container := BuildAppContainer()
	require.NotNil(t, container)
	require.NotNil(t, container.Mux)

	t.Run("Dispatch endpoints enforce authentication", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/dispatch/board", nil)
		rec := httptest.NewRecorder()

		container.Mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Courier endpoints enforce authentication", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/courier/parcels", nil)
		rec := httptest.NewRecorder()

		container.Mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusUnauthorized, rec.Code)
	})

	t.Run("Dispatcher can access dispatch board", func(t *testing.T) {
		token, err := container.TokenProv.GenerateAccessToken(identityApp.Principal{
			UserID: "0192e2b0-8c24-7df7-b087-0b1297e6e5a0",
			Email:  "dispatcher@swiftdrop.io",
			Role:   identityDomain.RoleDispatcher,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/dispatch/board", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		container.Mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err = json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Contains(t, resp, "data")
	})

	t.Run("Courier can access active manifest", func(t *testing.T) {
		token, err := container.TokenProv.GenerateAccessToken(identityApp.Principal{
			UserID: "0192e2b0-8c24-7df7-b087-0b1297e6e5a1",
			Email:  "courier@swiftdrop.io",
			Role:   identityDomain.RoleCourier,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/courier/parcels", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		container.Mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		err = json.NewDecoder(rec.Body).Decode(&resp)
		require.NoError(t, err)
		assert.Contains(t, resp, "data")
	})

	t.Run("Courier forbidden from accessing dispatcher endpoints", func(t *testing.T) {
		token, err := container.TokenProv.GenerateAccessToken(identityApp.Principal{
			UserID: "0192e2b0-8c24-7df7-b087-0b1297e6e5a1",
			Email:  "courier@swiftdrop.io",
			Role:   identityDomain.RoleCourier,
		})
		require.NoError(t, err)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/dispatch/board", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()

		container.Mux.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusForbidden, rec.Code)
	})
}

func TestDispatchAndDeliveryWiring_AgreementA2Preserved(t *testing.T) {
	origEnv := os.Getenv("APP_ENV")
	defer os.Setenv("APP_ENV", origEnv)
	_ = os.Setenv("APP_ENV", "local")
	_ = os.Setenv("DATABASE_URL", "postgres://none:none@127.0.0.1:54399/none?sslmode=disable")

	container := BuildAppContainer()

	// 1. Verify /health/live is unaffected
	reqLive := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	recLive := httptest.NewRecorder()
	container.Mux.ServeHTTP(recLive, reqLive)
	assert.Equal(t, http.StatusOK, recLive.Code)

	// 2. Verify /health/ready is unaffected (checks DB ping only, not storage)
	reqReady := httptest.NewRequest(http.MethodGet, "/health/ready", nil)
	recReady := httptest.NewRecorder()
	container.Mux.ServeHTTP(recReady, reqReady)
	assert.Equal(t, http.StatusOK, recReady.Code)
}

