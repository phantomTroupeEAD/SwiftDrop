package infrastructure

import (
	"context"
	"fmt"
	"html"
	"net/smtp"
)

// SMTPEmailSender implements application.EmailSender using standard SMTP (net/smtp).
// Compatible with Gmail SMTP, Brevo (Sendinblue), SendGrid, Mailgun, AWS SES, or any standard SMTP host.
type SMTPEmailSender struct {
	host      string
	port      string
	username  string
	password  string
	fromEmail string
	fromName  string
}

// NewSMTPEmailSender creates a configured SMTPEmailSender.
func NewSMTPEmailSender(host, port, username, password, fromEmail, fromName string) *SMTPEmailSender {
	if fromName == "" {
		fromName = "SwiftDrop Deliveries"
	}
	return &SMTPEmailSender{
		host:      host,
		port:      port,
		username:  username,
		password:  password,
		fromEmail: fromEmail,
		fromName:  fromName,
	}
}

// SendOTPEmail sends a formatted HTML email containing the 6-digit verification code.
func (s *SMTPEmailSender) SendOTPEmail(_ context.Context, recipientEmail, code string) error {
	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	// Auth with credentials if provided (some local relays like MailHog don't require auth)
	var auth smtp.Auth
	if s.username != "" || s.password != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	fromHeader := fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)
	subject := "Subject: SwiftDrop — Verify Your Email\r\n"
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n", fromHeader, recipientEmail)

	body := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #f4f5f7; margin: 0; padding: 24px; }
    .container { max-width: 520px; margin: 0 auto; background: #ffffff; border-radius: 8px; padding: 32px; box-shadow: 0 2px 4px rgba(0,0,0,0.05); }
    .logo { font-size: 22px; font-weight: bold; color: #1e3a8a; margin-bottom: 24px; }
    .title { font-size: 18px; font-weight: 600; color: #111827; margin-bottom: 12px; }
    .text { color: #4b5563; line-height: 1.5; font-size: 14px; margin-bottom: 24px; }
    .otp-box { background: #f0fdf4; border: 1px dashed #22c55e; border-radius: 6px; padding: 16px; text-align: center; margin-bottom: 24px; }
    .otp-code { font-size: 32px; font-weight: 800; letter-spacing: 6px; color: #15803d; font-family: monospace; }
    .footer { font-size: 12px; color: #9ca3af; text-align: center; margin-top: 24px; }
  </style>
</head>
<body>
  <div class="container">
    <div class="logo">⚡ SwiftDrop Logistics</div>
    <div class="title">Verify your email address</div>
    <div class="text">Welcome to SwiftDrop! Use the 6-digit verification code below to verify your email and finish setting up your account:</div>
    <div class="otp-box">
      <div class="otp-code">%s</div>
    </div>
    <div class="text">This code will expire in <strong>10 minutes</strong>. If you did not request this verification code, please ignore this email.</div>
    <div class="footer">&copy; SwiftDrop Logistics Platform. All rights reserved.</div>
  </div>
</body>
</html>`, html.EscapeString(code))

	msg := []byte(subject + headers + body)
	return smtp.SendMail(addr, auth, s.fromEmail, []string{recipientEmail}, msg)
}

// SendPasswordResetEmail sends a formatted HTML email containing the 6-digit password reset code.
func (s *SMTPEmailSender) SendPasswordResetEmail(_ context.Context, recipientEmail, code string) error {
	addr := fmt.Sprintf("%s:%s", s.host, s.port)

	var auth smtp.Auth
	if s.username != "" || s.password != "" {
		auth = smtp.PlainAuth("", s.username, s.password, s.host)
	}

	fromHeader := fmt.Sprintf("%s <%s>", s.fromName, s.fromEmail)
	subject := "Subject: SwiftDrop — Reset Your Password\r\n"
	headers := fmt.Sprintf("From: %s\r\nTo: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/html; charset=UTF-8\r\n\r\n", fromHeader, recipientEmail)

	body := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <style>
    body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Helvetica, Arial, sans-serif; background-color: #f4f5f7; margin: 0; padding: 24px; }
    .container { max-width: 520px; margin: 0 auto; background: #ffffff; border-radius: 8px; padding: 32px; box-shadow: 0 2px 4px rgba(0,0,0,0.05); }
    .logo { font-size: 22px; font-weight: bold; color: #1e3a8a; margin-bottom: 24px; }
    .title { font-size: 18px; font-weight: 600; color: #111827; margin-bottom: 12px; }
    .text { color: #4b5563; line-height: 1.5; font-size: 14px; margin-bottom: 24px; }
    .otp-box { background: #eff6ff; border: 1px dashed #3b82f6; border-radius: 6px; padding: 16px; text-align: center; margin-bottom: 24px; }
    .otp-code { font-size: 32px; font-weight: 800; letter-spacing: 6px; color: #1d4ed8; font-family: monospace; }
    .footer { font-size: 12px; color: #9ca3af; text-align: center; margin-top: 24px; }
  </style>
</head>
<body>
  <div class="container">
    <div class="logo">⚡ SwiftDrop Logistics</div>
    <div class="title">Reset your password</div>
    <div class="text">We received a request to reset your SwiftDrop account password. Use the 6-digit verification code below to set a new password:</div>
    <div class="otp-box">
      <div class="otp-code">%s</div>
    </div>
    <div class="text">This code will expire in <strong>10 minutes</strong>. If you did not request a password reset, you can safely ignore this email.</div>
    <div class="footer">&copy; SwiftDrop Logistics Platform. All rights reserved.</div>
  </div>
</body>
</html>`, html.EscapeString(code))

	msg := []byte(subject + headers + body)
	return smtp.SendMail(addr, auth, s.fromEmail, []string{recipientEmail}, msg)
}

// UnconfiguredEmailSender fails explicitly when SMTP_HOST is not configured.
type UnconfiguredEmailSender struct{}

// NewUnconfiguredEmailSender constructs an EmailSender that rejects email sends when SMTP is unconfigured.
func NewUnconfiguredEmailSender() *UnconfiguredEmailSender {
	return &UnconfiguredEmailSender{}
}

// SendOTPEmail returns an error indicating SMTP_HOST must be configured.
func (s *UnconfiguredEmailSender) SendOTPEmail(_ context.Context, _, _ string) error {
	return fmt.Errorf("email service not configured: SMTP_HOST environment variable is required")
}

// SendPasswordResetEmail returns an error indicating SMTP_HOST must be configured.
func (s *UnconfiguredEmailSender) SendPasswordResetEmail(_ context.Context, _, _ string) error {
	return fmt.Errorf("email service not configured: SMTP_HOST environment variable is required")
}
