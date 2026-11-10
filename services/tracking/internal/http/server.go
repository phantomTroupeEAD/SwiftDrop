package http

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"time"

	"github.com/swiftdrop/tracking/internal/config"
)

// Server wraps the standard HTTP server with lifecycle controls and timeouts.
type Server struct {
	httpServer *http.Server
	listener   net.Listener
	logger     *slog.Logger
}

// NewServer initializes a new Server configured with timeouts from Config.
func NewServer(cfg *config.Config, handler http.Handler, logger *slog.Logger) *Server {
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       cfg.ReadTimeout,
		WriteTimeout:      cfg.WriteTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	return &Server{
		httpServer: srv,
		logger:     logger,
	}
}

// Start binds to the configured network address and begins serving HTTP traffic in a background goroutine.
func (s *Server) Start() error {
	ln, err := net.Listen("tcp", s.httpServer.Addr)
	if err != nil {
		return fmt.Errorf("tracking server failed to listen on %s: %w", s.httpServer.Addr, err)
	}
	s.listener = ln

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			if s.logger != nil {
				s.logger.Error("Tracking HTTP server error", "error", err)
			}
		}
	}()

	return nil
}

// Addr returns the bound network address, particularly useful when port 0 was passed for ephemeral testing.
func (s *Server) Addr() net.Addr {
	if s.listener != nil {
		return s.listener.Addr()
	}
	return nil
}

// Shutdown gracefully stops the HTTP server within the provided context deadline.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer == nil {
		return nil
	}
	return s.httpServer.Shutdown(ctx)
}
