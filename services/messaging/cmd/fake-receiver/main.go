package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/swiftdrop/messaging/internal/webhook"
)

func main() {
	port := flag.Int("port", 8088, "HTTP port to listen on")
	secret := flag.String("secret", "", "Webhook signing secret (defaults to WEBHOOK_SIGNING_SECRET env)")
	flag.Parse()

	secStr := *secret
	if secStr == "" {
		secStr = os.Getenv("WEBHOOK_SIGNING_SECRET")
	}
	if secStr == "" {
		secStr = "whsec_test_secret_for_local_merchant_123"
	}

	secretBytes := []byte(secStr)

	http.HandleFunc("/webhook", func(w http.ResponseWriter, r *http.Request) {
		// Support fault injection via query parameter, e.g. /webhook?status=500
		if statusParam := r.URL.Query().Get("status"); statusParam != "" {
			if code, err := strconv.Atoi(statusParam); err == nil && code >= 200 && code < 600 {
				w.WriteHeader(code)
				_, _ = w.Write([]byte(fmt.Sprintf(`{"status":"fault_injected","code":%d}`, code)))
				log.Printf("[fake-receiver] Injected HTTP %d response", code)
				return
			}
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read request body", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		sigHeader := r.Header.Get("X-SwiftDrop-Signature")
		deliveryID := r.Header.Get("X-SwiftDrop-Delivery-ID")
		eventID := r.Header.Get("X-SwiftDrop-Event-ID")

		valid := webhook.VerifySignature(body, sigHeader, secretBytes, 5*time.Minute)

		if !valid {
			log.Printf("[fake-receiver] REJECTED invalid signature: delivery_id=%s event_id=%s sig=%s", deliveryID, eventID, sigHeader)
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"error":"invalid_signature"}`))
			return
		}

		log.Printf("[fake-receiver] ACCEPTED valid webhook: delivery_id=%s event_id=%s body_length=%d", deliveryID, eventID, len(body))
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"received","delivery_id":"` + deliveryID + `"}`))
	})

	addr := fmt.Sprintf(":%d", *port)
	log.Printf("[fake-receiver] Listening on http://localhost%s/webhook (secret=%s)", addr, secStr)
	srv := &http.Server{
		Addr:              addr,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatalf("[fake-receiver] Server error: %v", err)
	}
}
