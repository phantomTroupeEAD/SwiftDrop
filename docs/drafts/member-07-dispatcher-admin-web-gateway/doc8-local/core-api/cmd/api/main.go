// Command api starts the Core API process with the admin module mounted.
// In the real repo this file belongs to the whole team; only the admin lines are mine.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"swiftdrop.local/m7/core-api/internal/admin/application"
	"swiftdrop.local/m7/core-api/internal/admin/infrastructure"
	"swiftdrop.local/m7/core-api/internal/admin/transport"
	"swiftdrop.local/m7/core-api/internal/platform/apperr"
	"swiftdrop.local/m7/core-api/internal/platform/auth"
	"swiftdrop.local/m7/core-api/internal/platform/config"
	"swiftdrop.local/m7/core-api/internal/platform/obs"
	"swiftdrop.local/m7/core-api/internal/platform/web"
)

func main() {
	healthcheck := flag.Bool("healthcheck", false, "call /livez on this container and exit 0 or 1 (Docker HEALTHCHECK in a distroless image)")
	flag.Parse()
	if *healthcheck {
		os.Exit(runHealthcheck())
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "core-api: "+err.Error())
		os.Exit(1)
	}
}

func runHealthcheck() int {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" || addr[0] == ':' {
		addr = "127.0.0.1" + addr
		if addr == "127.0.0.1" {
			addr += ":8080"
		}
	}
	c := http.Client{Timeout: 2 * time.Second}
	resp, err := c.Get("http://" + addr + "/livez")
	if err != nil || resp.StatusCode != 200 {
		return 1
	}
	return 0
}

func run() error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return fmt.Errorf("bad configuration:\n%w", err) // fail fast, before opening anything
	}
	log := obs.NewLogger(os.Stdout, cfg.LogLevel)
	slog.SetDefault(log) // every package, including web.WriteError, logs the same JSON way
	log.Info("starting", "config", cfg.Safe())

	pemBytes, err := cfg.PublicKeyPEM()
	if err != nil {
		return fmt.Errorf("JWT_PUBLIC_KEY_PEM: %w", err)
	}
	pub, err := auth.ParsePublicKey(pemBytes)
	if err != nil {
		return err
	}

	pcfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return errors.New("DATABASE_URL is not a valid PostgreSQL URL") // never echo the URL: it holds the password
	}
	pcfg.MaxConns, pcfg.MinConns = cfg.DBMaxOpenConns, cfg.DBMinIdleConns
	pcfg.ConnConfig.RuntimeParams["statement_timeout"] = fmt.Sprint(cfg.DBStatementTO.Milliseconds())
	pcfg.ConnConfig.RuntimeParams["application_name"] = "core-api"
	pcfg.ConnConfig.ConnectTimeout = 5 * time.Second
	pool, err := pgxpool.NewWithConfig(context.Background(), pcfg) // lazy: does not fail if the DB starts later
	if err != nil {
		return err
	}
	defer pool.Close()

	metrics := obs.NewMetrics()
	metrics.Gauge("db_pool_in_use_connections", "Connections busy right now.", func() float64 { return float64(pool.Stat().AcquiredConns()) })
	metrics.Gauge("db_pool_max_connections", "Pool size (DB_MAX_OPEN_CONNS).", func() float64 { return float64(pool.Stat().MaxConns()) })
	metrics.Gauge("db_pool_wait_total", "Times a request had to wait for a free connection.", func() float64 { return float64(pool.Stat().EmptyAcquireCount()) })

	var shuttingDown atomic.Bool
	mux := http.NewServeMux()
	mux.Handle(obs.Route("GET /livez", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"UP"}`))
	})))
	mux.Handle(obs.Route("GET /readyz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		switch {
		case shuttingDown.Load():
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"status":"SHUTTING_DOWN"}`))
		case pool.Ping(ctx) != nil:
			w.WriteHeader(503)
			_, _ = w.Write([]byte(`{"status":"NOT_READY","database":"DOWN"}`))
		default:
			_, _ = w.Write([]byte(`{"status":"READY","database":"UP"}`))
		}
	})))
	mux.Handle(obs.Route("GET /metrics", metrics))
	svc := &application.Service{Store: &infrastructure.Store{Pool: pool, LockTimeout: cfg.AdminLockTO}, NewID: newUUID}
	(&transport.Handlers{Svc: svc, Metrics: metrics}).Register(mux)
	// Anything else, including /api/v2/..., is a JSON 404 (Doc 7 GS-04).
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { web.WriteError(w, r, apperr.ErrNotFound) })

	verifier := auth.NewVerifier(pub, cfg.JWTIssuer)
	handler := verifier.Middleware(obs.Middleware(log, metrics, withTimeout(cfg.RequestTimeout, mux)))
	srv := &http.Server{
		Addr: cfg.HTTPAddr, Handler: handler,
		ReadHeaderTimeout: 10 * time.Second, ReadTimeout: 30 * time.Second,
		WriteTimeout: cfg.RequestTimeout + 5*time.Second, IdleTimeout: 120 * time.Second,
		ErrorLog: slog.NewLogLogger(log.Handler(), slog.LevelWarn),
	}

	errc := make(chan error, 1)
	go func() { errc <- srv.ListenAndServe() }()
	log.Info("listening", "addr", cfg.HTTPAddr)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGTERM, syscall.SIGINT)
	select {
	case err := <-errc:
		return err
	case sig := <-stop:
		// 1) Say "not ready" so Kubernetes and Traefik stop sending new requests.
		shuttingDown.Store(true)
		log.Info("shutdown started", "signal", sig.String(), "drain", cfg.ReadinessDrain.String())
		time.Sleep(cfg.ReadinessDrain)
		// 2) Finish requests already running, up to the grace period.
		ctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownGrace)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			log.Error("shutdown timed out, closing open requests", "err", err)
		}
		// 3) Close the database pool last (deferred above).
		log.Info("shutdown complete")
		return nil
	}
}

// withTimeout gives each request a deadline below the gateway's 30 s, so the
// database work is cancelled before the gateway gives up (Doc 7 section 7.1).
func withTimeout(d time.Duration, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), d)
		defer cancel()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func newUUID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:]
}
