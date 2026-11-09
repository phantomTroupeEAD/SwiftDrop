package notification

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/swiftdrop/messaging/internal/config"
	"github.com/swiftdrop/messaging/internal/delivery"
)

// fakeSMTPServer listens on a local port and responds with customizable SMTP responses.
type fakeSMTPServer struct {
	ln       net.Listener
	addr     string
	respCode int
	received []string
}

func startFakeSMTPServer(t *testing.T, respCode int) *fakeSMTPServer {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind fake smtp server: %v", err)
	}

	server := &fakeSMTPServer{
		ln:       ln,
		addr:     ln.Addr().String(),
		respCode: respCode,
	}

	go server.serve()
	return server
}

func (s *fakeSMTPServer) port() int {
	return s.ln.Addr().(*net.TCPAddr).Port
}

func (s *fakeSMTPServer) close() {
	_ = s.ln.Close()
}

func (s *fakeSMTPServer) serve() {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			return
		}
		go s.handleConn(conn)
	}
}

func (s *fakeSMTPServer) handleConn(conn net.Conn) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	// Send initial greeting
	_, _ = writer.WriteString("220 swiftdrop.local Service ready\r\n")
	_ = writer.Flush()

	inData := false

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return
		}
		trimmed := strings.TrimRight(line, "\r\n")

		if inData {
			s.received = append(s.received, trimmed)
			if trimmed == "." {
				inData = false
				if s.respCode >= 500 {
					_, _ = writer.WriteString(fmt.Sprintf("%d 5.0.0 Permanent failure\r\n", s.respCode))
				} else if s.respCode >= 400 {
					_, _ = writer.WriteString(fmt.Sprintf("%d 4.0.0 Temporary failure\r\n", s.respCode))
				} else {
					_, _ = writer.WriteString("250 2.0.0 OK queued as 12345\r\n")
				}
				_ = writer.Flush()
			}
			continue
		}

		upper := strings.ToUpper(trimmed)
		switch {
		case strings.HasPrefix(upper, "EHLO") || strings.HasPrefix(upper, "HELO"):
			_, _ = writer.WriteString("250-swiftdrop.local\r\n250 HELP\r\n")
			_ = writer.Flush()
		case strings.HasPrefix(upper, "MAIL FROM:"):
			_, _ = writer.WriteString("250 2.1.0 Sender OK\r\n")
			_ = writer.Flush()
		case strings.HasPrefix(upper, "RCPT TO:"):
			_, _ = writer.WriteString("250 2.1.5 Recipient OK\r\n")
			_ = writer.Flush()
		case upper == "DATA":
			_, _ = writer.WriteString("354 Start mail input; end with <CRLF>.<CRLF>\r\n")
			_ = writer.Flush()
			inData = true
		case upper == "QUIT":
			_, _ = writer.WriteString("221 2.0.0 Bye\r\n")
			_ = writer.Flush()
			return
		default:
			_, _ = writer.WriteString("250 OK\r\n")
			_ = writer.Flush()
		}
	}
}

func TestSMTPSender_Success250(t *testing.T) {
	srv := startFakeSMTPServer(t, 250)
	defer srv.close()

	cfg := &config.Config{
		AppEnv:        "local",
		SMTPHost:      "127.0.0.1",
		SMTPPort:      srv.port(),
		SMTPSecurity:  "none",
		SMTPFromEmail: "no-reply@swiftdrop.local",
		SMTPFromName:  "SwiftDrop",
	}

	sender := NewSMTPSender(cfg)
	msgID, err := sender.Send(context.Background(), "customer@example.lk", "Test Subject", "Hello Text", "<p>Hello HTML</p>")
	if err != nil {
		t.Fatalf("unexpected send error: %v", err)
	}

	if !strings.HasPrefix(msgID, "<") || !strings.HasSuffix(msgID, "@swiftdrop>") {
		t.Errorf("unexpected message-id format: %s", msgID)
	}
}

func TestSMTPSender_CRLFInjectionBlocked(t *testing.T) {
	cfg := &config.Config{
		AppEnv:        "local",
		SMTPHost:      "127.0.0.1",
		SMTPPort:      1025,
		SMTPSecurity:  "none",
		SMTPFromEmail: "no-reply@swiftdrop.local",
		SMTPFromName:  "SwiftDrop",
	}

	sender := NewSMTPSender(cfg)

	// Inject newline in toEmail
	_, err := sender.Send(context.Background(), "victim@example.lk\r\nBcc: evil@attacker.com", "Test", "Text", "HTML")
	if err != ErrHeaderInjection {
		t.Errorf("expected ErrHeaderInjection on newline in email, got %v", err)
	}

	// Inject newline in subject
	_, err = sender.Send(context.Background(), "victim@example.lk", "Test\nSubject", "Text", "HTML")
	if err != ErrHeaderInjection {
		t.Errorf("expected ErrHeaderInjection on newline in subject, got %v", err)
	}
}

func TestSMTPSender_ErrorClassification(t *testing.T) {
	if !IsTerminalSMTPError(ErrHeaderInjection) {
		t.Errorf("expected ErrHeaderInjection to be terminal")
	}

	err550 := fmt.Errorf("smtp data failed: 550 User unknown")
	if !IsTerminalSMTPError(err550) {
		t.Errorf("expected 550 error to be terminal")
	}

	err451 := fmt.Errorf("smtp data failed: 451 Temporary failure")
	if IsTerminalSMTPError(err451) {
		t.Errorf("expected 451 error to NOT be terminal")
	}
}

func TestTemplates_RenderAllMilestones(t *testing.T) {
	milestones := []string{
		"parcel.booked.v1",
		"parcel.out_for_delivery.v1",
		"parcel.delivered.v1",
		"parcel.delivery_failed.v1",
		"parcel.returned.v1",
	}

	data := MilestoneData{
		RecipientName:  "Nimal Silva",
		TrackingNumber: "SD-20261101-ABCD",
		CourierName:    "Kamal",
		Reason:         "Premises locked",
	}

	for _, m := range milestones {
		t.Run(m, func(t *testing.T) {
			subj, text, html, err := RenderNotification(m, data)
			if err != nil {
				t.Fatalf("failed to render milestone %s: %v", m, err)
			}

			if !strings.Contains(subj, data.TrackingNumber) {
				t.Errorf("subject missing tracking number: %s", subj)
			}
			if !strings.Contains(text, data.RecipientName) {
				t.Errorf("text missing recipient name: %s", text)
			}
			if !strings.Contains(html, data.RecipientName) {
				t.Errorf("html missing recipient name: %s", html)
			}

			// Verify no internal UUID or dummy string was leaked
			if strings.Contains(text, "0192e2b3-") {
				t.Errorf("text leaked UUID: %s", text)
			}
		})
	}
}

func TestNotificationPoller_Success(t *testing.T) {
	repo := newMockNotificationRepo()
	sender := &mockEmailSender{}
	cfg := &config.Config{
		DeliveryLeaseSeconds: 60,
	}

	calc := delivery.NewBackoffCalculator(delivery.DefaultRetrySchedule, nil)
	poller := NewPoller(repo, sender, calc, cfg, nil)

	repo.tasks = append(repo.tasks, delivery.NotificationTask{
		ID:        "notif-1",
		EventID:   "0192e2b3-7c8a-7890-a123-000000000001",
		EventType: "parcel.booked.v1",
		Recipient: "nimal@example.lk",
		Payload:   []byte(`{"tracking_number":"SD-001","notify":{"recipient_name":"Nimal"}}`),
	})

	count, err := poller.PollAndSend(context.Background())
	if err != nil {
		t.Fatalf("unexpected poller error: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected 1 task processed, got %d", count)
	}

	if repo.successID != "notif-1" {
		t.Errorf("expected success recorded for notif-1, got %s", repo.successID)
	}
}

type mockEmailSender struct {
	sendErr error
}

func (m *mockEmailSender) Send(ctx context.Context, toEmail, subject, textBody, htmlBody string) (string, error) {
	if m.sendErr != nil {
		return "", m.sendErr
	}
	return "<test-msg-id@swiftdrop>", nil
}

type mockNotificationRepo struct {
	delivery.Repository
	tasks     []delivery.NotificationTask
	successID string
	retryID   string
	failID    string
}

func newMockNotificationRepo() *mockNotificationRepo {
	return &mockNotificationRepo{}
}

func (m *mockNotificationRepo) ClaimNotifications(ctx context.Context, limit int, leaseDuration time.Duration) ([]delivery.NotificationTask, error) {
	res := m.tasks
	m.tasks = nil
	return res, nil
}

func (m *mockNotificationRepo) RecordNotificationSuccess(ctx context.Context, id, eventID, providerMessageID string) error {
	m.successID = id
	return nil
}

func (m *mockNotificationRepo) RecordNotificationRetry(ctx context.Context, id string, lastError string, nextAttemptAt time.Time) error {
	m.retryID = id
	return nil
}

func (m *mockNotificationRepo) RecordNotificationTerminalFailure(ctx context.Context, id, eventID string, lastError string) error {
	m.failID = id
	return nil
}
