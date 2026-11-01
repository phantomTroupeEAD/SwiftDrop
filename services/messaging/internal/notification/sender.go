package notification

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/smtp"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/swiftdrop/messaging/internal/config"
)

var (
	ErrHeaderInjection  = errors.New("notification: header contains illegal newline character")
	ErrSTARTTLSRequired = errors.New("notification: SMTP server does not support required STARTTLS")
)

// EmailSender defines the interface for delivering customer emails.
type EmailSender interface {
	Send(ctx context.Context, toEmail, subject, textBody, htmlBody string) (providerMsgID string, err error)
}

// SMTPSender implements EmailSender using standard library net/smtp.
type SMTPSender struct {
	cfg *config.Config
}

// NewSMTPSender creates a new SMTPSender.
func NewSMTPSender(cfg *config.Config) *SMTPSender {
	return &SMTPSender{cfg: cfg}
}

func generateMessageID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("<%s@swiftdrop>", hex.EncodeToString(b[:]))
}

func containsNewline(s string) bool {
	return strings.ContainsAny(s, "\r\n")
}

// Send formats a MIME multipart/alternative message and transmits it to the SMTP host.
func (s *SMTPSender) Send(ctx context.Context, toEmail, subject, textBody, htmlBody string) (string, error) {
	// Guard against CRLF injection in all headers
	if containsNewline(toEmail) || containsNewline(subject) || containsNewline(s.cfg.SMTPFromEmail) || containsNewline(s.cfg.SMTPFromName) {
		return "", ErrHeaderInjection
	}

	msgID := generateMessageID()
	boundary := "===============swiftdrop_mime_boundary=="

	fromHeader := fmt.Sprintf("%s <%s>", s.cfg.SMTPFromName, s.cfg.SMTPFromEmail)
	encodedSubject := mime.QEncoding.Encode("utf-8", subject)

	var msgBuf strings.Builder
	msgBuf.WriteString(fmt.Sprintf("From: %s\r\n", fromHeader))
	msgBuf.WriteString(fmt.Sprintf("To: %s\r\n", toEmail))
	msgBuf.WriteString(fmt.Sprintf("Subject: %s\r\n", encodedSubject))
	msgBuf.WriteString(fmt.Sprintf("Date: %s\r\n", time.Now().Format(time.RFC1123Z)))
	msgBuf.WriteString(fmt.Sprintf("Message-ID: %s\r\n", msgID))
	msgBuf.WriteString("MIME-Version: 1.0\r\n")
	msgBuf.WriteString(fmt.Sprintf("Content-Type: multipart/alternative; boundary=\"%s\"\r\n", boundary))
	msgBuf.WriteString("\r\n")

	// Plain text part
	msgBuf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msgBuf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	msgBuf.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	msgBuf.WriteString(textBody)
	msgBuf.WriteString("\r\n\r\n")

	// HTML part
	msgBuf.WriteString(fmt.Sprintf("--%s\r\n", boundary))
	msgBuf.WriteString("Content-Type: text/html; charset=utf-8\r\n")
	msgBuf.WriteString("Content-Transfer-Encoding: 8bit\r\n\r\n")
	msgBuf.WriteString(htmlBody)
	msgBuf.WriteString("\r\n\r\n")

	// Boundary end
	msgBuf.WriteString(fmt.Sprintf("--%s--\r\n", boundary))

	rawMsg := []byte(msgBuf.String())
	addr := fmt.Sprintf("%s:%d", s.cfg.SMTPHost, s.cfg.SMTPPort)

	// Dial and execute with context deadline
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	var client *smtp.Client
	var err error

	switch s.cfg.SMTPSecurity {
	case "tls":
		tlsConn, dialErr := tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
			ServerName: s.cfg.SMTPHost,
			MinVersion: tls.VersionTLS12,
		})
		if dialErr != nil {
			return "", fmt.Errorf("tls dial failed: %w", dialErr)
		}
		client, err = smtp.NewClient(tlsConn, s.cfg.SMTPHost)
		if err != nil {
			_ = tlsConn.Close()
			return "", fmt.Errorf("smtp new client failed: %w", err)
		}

	case "starttls":
		conn, dialErr := dialer.DialContext(ctx, "tcp", addr)
		if dialErr != nil {
			return "", fmt.Errorf("tcp dial failed: %w", dialErr)
		}
		client, err = smtp.NewClient(conn, s.cfg.SMTPHost)
		if err != nil {
			_ = conn.Close()
			return "", fmt.Errorf("smtp new client failed: %w", err)
		}
		defer client.Close()

		hasStartTLS, _ := client.Extension("STARTTLS")
		if !hasStartTLS {
			return "", ErrSTARTTLSRequired
		}
		if err := client.StartTLS(&tls.Config{ServerName: s.cfg.SMTPHost, MinVersion: tls.VersionTLS12}); err != nil {
			return "", fmt.Errorf("starttls negotiation failed: %w", err)
		}

	default: // "none"
		conn, dialErr := dialer.DialContext(ctx, "tcp", addr)
		if dialErr != nil {
			return "", fmt.Errorf("tcp dial failed: %w", dialErr)
		}
		client, err = smtp.NewClient(conn, s.cfg.SMTPHost)
		if err != nil {
			_ = conn.Close()
			return "", fmt.Errorf("smtp new client failed: %w", err)
		}
	}
	defer client.Close()

	// Authenticate if credentials provided
	if s.cfg.SMTPUsername != "" {
		auth := smtp.PlainAuth("", s.cfg.SMTPUsername, s.cfg.SMTPPassword, s.cfg.SMTPHost)
		if err := client.Auth(auth); err != nil {
			return "", fmt.Errorf("smtp auth failed: %w", err)
		}
	}

	// Send MAIL FROM
	if err := client.Mail(s.cfg.SMTPFromEmail); err != nil {
		return "", fmt.Errorf("smtp mail from failed: %w", err)
	}

	// Send RCPT TO
	if err := client.Rcpt(toEmail); err != nil {
		return "", fmt.Errorf("smtp rcpt to failed: %w", err)
	}

	// Send DATA
	w, err := client.Data()
	if err != nil {
		return "", fmt.Errorf("smtp data failed: %w", err)
	}
	if _, err := w.Write(rawMsg); err != nil {
		_ = w.Close()
		return "", fmt.Errorf("smtp write message failed: %w", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("smtp close data failed: %w", err)
	}

	_ = client.Quit()
	return msgID, nil
}

var smtpStatusCodeRegex = regexp.MustCompile(`\b([45]\d{2})\b`)

// IsTerminalSMTPError determines if an error is a permanent 5xx failure.
func IsTerminalSMTPError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrHeaderInjection) {
		return true
	}

	matches := smtpStatusCodeRegex.FindStringSubmatch(err.Error())
	if len(matches) >= 2 {
		code, parseErr := strconv.Atoi(matches[1])
		if parseErr == nil && code >= 500 && code < 600 {
			return true
		}
	}

	return false
}
