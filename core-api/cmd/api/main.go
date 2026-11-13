package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// @title                      SwiftDrop Core API
// @version                    1.0
// @description                Authoritative REST API Modular Monolith for SwiftDrop Courier & Parcel Delivery Management System.
// @termsOfService             https://swiftdrop.local/terms

// @contact.name               SwiftDrop Engineering Team
// @contact.url                https://github.com/swiftdrop

// @license.name               Proprietary / Academic Use Only

// @host                       localhost:8080
// @BasePath                   /

// @securityDefinitions.apikey BearerAuth
// @in                          header
// @name                        Authorization
// @description                 Type "Bearer" followed by a space and JWT token (e.g. 'Bearer eyJhbGci...').

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	container := BuildAppContainer()
	if container.DB == nil {
		log.Fatal("[FATAL] PostgreSQL database connection is required to start SwiftDrop Core API. Set DATABASE_URL and ensure PostgreSQL is running.")
	}
	if os.Getenv("JWT_SECRET") == "" {
		log.Fatal("[FATAL] JWT_SECRET environment variable is required.")
	}

	fmt.Println("==================================================================")
	fmt.Println("   SwiftDrop Core API (Modular Monolith) — Identity & Auth Ready")
	fmt.Println("   Storage:", container.StorageType)
	fmt.Println("==================================================================")
	fmt.Println(" Auth REST API Endpoints:")
	fmt.Println(" - POST http://localhost:" + port + "/api/v1/auth/login")
	fmt.Println(" - GET  http://localhost:" + port + "/api/v1/auth/me (Bearer Token)")
	fmt.Println(" - GET  http://localhost:" + port + "/api/v1/admin/dashboard (Admin only)")
	fmt.Println(" - GET  http://localhost:" + port + "/api/v1/courier/dashboard (Courier/Admin)")
	fmt.Println(" - GET  http://localhost:" + port + "/api/v1/merchant/portal (Merchant/Admin)")
	fmt.Println("==================================================================")

	serverAddr := ":" + port
	srv := &http.Server{
		Addr:              serverAddr,
		Handler:           wrapGlobalMiddleware(container.Mux),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Graceful shutdown handling on SIGTERM / SIGINT per Phase 8B
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("Server listening on http://localhost:%s\n", port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	<-stop
	log.Println("Shutting down server gracefully...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server graceful shutdown error: %v", err)
	}

	if container.DB != nil {
		if err := container.DB.Close(); err != nil {
			log.Printf("Error closing database connection: %v", err)
		} else {
			log.Println("Database connection pool closed.")
		}
	}

	log.Println("Server exited successfully.")
}
