// Command api runs the core-api process with the merchants module (local slice).
//
//	api               start the server
//	api -healthcheck  exit 0 if GET /livez on HTTP_ADDR returns 200 (Docker HEALTHCHECK, no shell needed)
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"sync/atomic"
	"syscall"
	"time"

	"swiftdrop.local/m4/core-api/internal/merchants/application"
	"swiftdrop.local/m4/core-api/internal/merchants/infrastructure"
	"swiftdrop.local/m4/core-api/internal/merchants/transport"
	"swiftdrop.local/m4/core-api/internal/platform/auth"
	"swiftdrop.local/m4/core-api/internal/platform/config"
	"swiftdrop.local/m4/core-api/internal/platform/db"
	"swiftdrop.local/m4/core-api/internal/platform/obs"
	"swiftdrop.local/m4/core-api/internal/platform/web"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		os.Exit(healthcheck())
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "core-api:", err)
		os.Exit(1)
	}
}

func healthcheck() int {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	if strings.HasPrefix(addr, ":") {
		addr = "127.0.0.1" + addr
	}
	c := http.Client{Timeout: 2 * time.Second}
	res, err := c.Get("http://" + addr + "/livez")
	if err != nil || res.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}

type observer struct{ m *obs.Metrics }

func (o observer) AuthResult(r string)    { o.m.Inc("merchant_api_auth_total", "result", r) }
func (o observer) SSRFBlocked()           { o.m.Inc("webhook_ssrf_blocked_total") }
func (o observer) StatusChanged(a string) { o.m.Inc("merchant_status_changes_total", "action", a) }

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err // lists every problem, never values (FM-05, FM-07)
	}
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel})).
		With("service", "core-api", "module", "merchants")

	pub, err := auth.ParsePublicKey(cfg.JWTPublicKeyPEM)
	if err != nil {
		return fmt.Errorf("JWT_PUBLIC_KEY_PEM: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	m := obs.New()
	m.Describe("merchant_api_auth_total", "Merchant API key authentication results")
	m.Describe("webhook_ssrf_blocked_total", "Webhook URLs rejected by the SSRF policy")
	m.Describe("merchant_status_changes_total", "Admin merchant status changes")
	m.Describe("http_requests_total", "HTTP requests by route and status")
	m.Describe("http_request_duration_seconds_sum", "Total request time by route")

	svc := application.NewService(application.Deps{
		Tx:        db.TxRunner{Pool: pool},
		Merchants: infrastructure.MerchantRepo{},
		Creds:     infrastructure.CredentialRepo{},
		Hooks:     infrastructure.WebhookRepo{},
		Audit:     infrastructure.AuditWriter{},
		Roles:     infrastructure.RoleAssigner{},
		KeyGen:    infrastructure.KeyGenerator{},
		Hasher:    infrastructure.Hasher{Peppers: map[int][]byte{cfg.PepperVersion: cfg.Pepper}, Current: cfg.PepperVersion},
		Cipher:    infrastructure.Cipher{Key: cfg.EncKey, Version: cfg.EncKeyVersion},
		Resolver:  infrastructure.Resolver{},
		Booking:   infrastructure.NewBookingStub(),
		Observer:  observer{m},
	}, application.Config{
		KeyEnvPrefix: cfg.KeyEnvPrefix, MaxActiveKeys: cfg.MaxActiveKeys, MaxWebhooks: cfg.MaxWebhooks,
		AllowHTTP: cfg.WebhookAllowHTTP, AllowedPrivateHosts: cfg.WebhookAllowedPrivateHosts,
	})

	var draining atomic.Bool
	mux := http.NewServeMux()
	mux.HandleFunc("GET /livez", func(w http.ResponseWriter, _ *http.Request) {
		web.JSON(w, http.StatusOK, map[string]string{"status": "UP"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		if draining.Load() {
			web.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "DRAINING"})
			return
		}
		pctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if err := pool.Ping(pctx); err != nil {
			web.JSON(w, http.StatusServiceUnavailable, map[string]string{"status": "NOT_READY", "database": "DOWN"})
			return
		}
		web.JSON(w, http.StatusOK, map[string]string{"status": "READY", "database": "UP"})
	})
	mux.Handle("GET /metrics", m.Handler())
	(&transport.Handler{Svc: svc, Verifier: auth.NewVerifier(pub, cfg.JWTIssuer), Log: log}).Register(mux)

	routeOf := func(r *http.Request) string {
		_, pattern := mux.Handler(r)
		return pattern
	}
	onDone := func(route string, status int, d time.Duration) {
		m.Inc("http_requests_total", "route", route, "status", fmt.Sprint(status))
		m.Add("http_request_duration_seconds_sum", d.Seconds(), "route", route)
	}
	srv := &http.Server{
		Addr: cfg.HTTPAddr, Handler: web.Middleware(log, routeOf, onDone, mux),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 15 * time.Second,
		IdleTimeout: 60 * time.Second, MaxHeaderBytes: 16 << 10,
	}
	errc := make(chan error, 1)
	go func() {
		log.Info("server started", "addr", cfg.HTTPAddr, "app_env", cfg.AppEnv, "key_prefix_env", cfg.KeyEnvPrefix)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errc <- err
		}
	}()

	select {
	case err := <-errc:
		return err
	case <-ctx.Done():
	}
	// Graceful shutdown: fail readiness, wait for the load balancer, finish in-flight requests.
	log.Info("shutdown started", "drain_seconds", cfg.ShutdownDrain.Seconds())
	draining.Store(true)
	time.Sleep(cfg.ShutdownDrain)
	sctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		log.Error("shutdown incomplete", "error", err.Error())
		return err
	}
	log.Info("shutdown complete")
	return nil
}
