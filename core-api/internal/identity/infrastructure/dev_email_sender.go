package infrastructure

import (
	"context"
	"fmt"
	"log"
	"time"
)

// DevEmailSender is a development-only EmailSender that prints the OTP to
// the server console instead of calling an external SMTP/API provider.
// Replace with a real provider (SendGrid, Mailgun, AWS SES) in production.
type DevEmailSender struct{}

// NewDevEmailSender constructs a DevEmailSender.
func NewDevEmailSender() *DevEmailSender {
	return &DevEmailSender{}
}

// SendOTPEmail logs the OTP prominently so developers can copy it from the terminal.
func (s *DevEmailSender) SendOTPEmail(_ context.Context, recipientEmail, code string) error {
	log.Println("╔══════════════════════════════════════════════════════╗")
	log.Println("║            SwiftDrop — DEV EMAIL (OTP)              ║")
	log.Println("╠══════════════════════════════════════════════════════╣")
	log.Printf("║  To:        %-40s ║\n", recipientEmail)
	log.Printf("║  Code:      %-40s ║\n", code)
	log.Printf("║  Sent at:   %-40s ║\n", time.Now().UTC().Format(time.RFC3339))
	log.Println("║  Expires:   10 minutes                               ║")
	log.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println() // blank line for readability
	return nil
}

// SendPasswordResetEmail logs the password reset OTP prominently so developers can copy it from the terminal.
func (s *DevEmailSender) SendPasswordResetEmail(_ context.Context, recipientEmail, code string) error {
	log.Println("╔══════════════════════════════════════════════════════╗")
	log.Println("║       SwiftDrop — DEV EMAIL (PASSWORD RESET)         ║")
	log.Println("╠══════════════════════════════════════════════════════╣")
	log.Printf("║  To:        %-40s ║\n", recipientEmail)
	log.Printf("║  Code:      %-40s ║\n", code)
	log.Printf("║  Sent at:   %-40s ║\n", time.Now().UTC().Format(time.RFC3339))
	log.Println("║  Expires:   10 minutes                               ║")
	log.Println("╚══════════════════════════════════════════════════════╝")
	fmt.Println() // blank line for readability
	return nil
}
