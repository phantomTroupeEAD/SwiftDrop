package broker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

var (
	// ErrNotConnected is returned when attempting to publish while disconnected.
	ErrNotConnected = errors.New("amqp broker: not connected")
	// ErrPublishNack is returned when RabbitMQ nacks a published message.
	ErrPublishNack = errors.New("amqp broker: message nacked by broker")
	// ErrUnroutable is returned when a mandatory message cannot be routed to any queue.
	ErrUnroutable = errors.New("amqp broker: message returned as unroutable")
)

// Manager manages a resilient RabbitMQ connection, channel, and topology.
type Manager struct {
	url    string
	logger *slog.Logger

	mu           sync.RWMutex
	conn         *amqp.Connection
	pubCh        *amqp.Channel
	confirms     chan amqp.Confirmation
	returns      chan amqp.Return
	closed       bool
	reconnecting bool
	notifyReady  chan struct{}
}

// NewManager creates a new RabbitMQ connection manager.
func NewManager(url string, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		url:         url,
		logger:      logger,
		notifyReady: make(chan struct{}),
	}
}

// Connect dials RabbitMQ, declares topology, and configures publisher confirms.
// It retries with exponential backoff until connected, the context expires, or Close is called.
// If the connection cannot be established before ctx expires, a background retry loop
// is started so the manager connects as soon as the broker becomes reachable.
func (m *Manager) Connect(ctx context.Context) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return errors.New("amqp broker manager is closed")
	}
	if m.conn != nil && !m.conn.IsClosed() {
		m.mu.RUnlock()
		return nil
	}
	m.mu.RUnlock()

	// 1. Try initial connection immediately
	conn, pubCh, confirms, returns, err := m.dialAndSetup()
	if err == nil {
		m.mu.Lock()
		if m.closed {
			m.mu.Unlock()
			_ = pubCh.Close()
			_ = conn.Close()
			return errors.New("amqp broker manager is closed")
		}
		m.conn = conn
		m.pubCh = pubCh
		m.confirms = confirms
		m.returns = returns
		m.reconnecting = false
		select {
		case <-m.notifyReady:
		default:
			close(m.notifyReady)
		}
		m.mu.Unlock()

		m.logger.Info("connected to rabbitmq and declared topology")
		go m.reconnectLoop(conn.NotifyClose(make(chan *amqp.Error, 1)))
		return nil
	}

	// 2. Initial dial failed; retry within ctx window with exponential backoff
	backoff := 500 * time.Millisecond
	maxBackoff := 5 * time.Second

	for {
		select {
		case <-ctx.Done():
			// Context expired before connection succeeded.
			// Start background retry loop so manager connects as soon as broker is ready.
			m.mu.Lock()
			if !m.closed && !m.reconnecting {
				m.reconnecting = true
				go m.startRetryLoop()
			}
			m.mu.Unlock()
			return fmt.Errorf("initial broker connect failed (retrying in background): %w: %v", ctx.Err(), err)
		case <-time.After(backoff):
		}

		m.mu.RLock()
		if m.closed {
			m.mu.RUnlock()
			return errors.New("amqp broker manager is closed")
		}
		if m.conn != nil && !m.conn.IsClosed() {
			m.mu.RUnlock()
			return nil
		}
		m.mu.RUnlock()

		conn, pubCh, confirms, returns, err = m.dialAndSetup()
		if err == nil {
			m.mu.Lock()
			if m.closed {
				m.mu.Unlock()
				_ = pubCh.Close()
				_ = conn.Close()
				return errors.New("amqp broker manager is closed")
			}
			m.conn = conn
			m.pubCh = pubCh
			m.confirms = confirms
			m.returns = returns
			m.reconnecting = false
			select {
			case <-m.notifyReady:
			default:
				close(m.notifyReady)
			}
			m.mu.Unlock()

			m.logger.Info("connected to rabbitmq and declared topology")
			go m.reconnectLoop(conn.NotifyClose(make(chan *amqp.Error, 1)))
			return nil
		}

		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// dialAndSetup establishes a connection, declares topology, and configures confirm mode without holding the manager lock.
func (m *Manager) dialAndSetup() (*amqp.Connection, *amqp.Channel, chan amqp.Confirmation, chan amqp.Return, error) {
	conn, err := amqp.DialConfig(m.url, amqp.Config{
		Dial: amqp.DefaultDial(5 * time.Second),
	})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("failed to dial rabbitmq: %w", err)
	}

	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, nil, nil, nil, fmt.Errorf("failed to open publisher channel: %w", err)
	}

	// 1. Declare Topology idempotently
	if err := DeclareTopology(pubCh); err != nil {
		_ = pubCh.Close()
		_ = conn.Close()
		return nil, nil, nil, nil, fmt.Errorf("failed to declare broker topology: %w", err)
	}

	// 2. Put publisher channel into confirm mode
	if err := pubCh.Confirm(false); err != nil {
		_ = pubCh.Close()
		_ = conn.Close()
		return nil, nil, nil, nil, fmt.Errorf("failed to put channel into confirm mode: %w", err)
	}

	confirms := pubCh.NotifyPublish(make(chan amqp.Confirmation, 100))
	returns := pubCh.NotifyReturn(make(chan amqp.Return, 100))

	return conn, pubCh, confirms, returns, nil
}

// startRetryLoop repeatedly attempts to connect until successful or the manager is closed.
func (m *Manager) startRetryLoop() {
	backoff := 500 * time.Millisecond
	maxBackoff := 10 * time.Second

	for {
		m.mu.RLock()
		if m.closed {
			m.mu.RUnlock()
			return
		}
		if m.conn != nil && !m.conn.IsClosed() {
			m.mu.RUnlock()
			return
		}
		m.mu.RUnlock()

		time.Sleep(backoff)

		m.mu.RLock()
		if m.closed {
			m.mu.RUnlock()
			return
		}
		m.mu.RUnlock()

		conn, pubCh, confirms, returns, err := m.dialAndSetup()
		if err == nil {
			m.mu.Lock()
			if m.closed {
				m.mu.Unlock()
				_ = pubCh.Close()
				_ = conn.Close()
				return
			}
			m.conn = conn
			m.pubCh = pubCh
			m.confirms = confirms
			m.returns = returns
			m.reconnecting = false

			select {
			case <-m.notifyReady:
			default:
				close(m.notifyReady)
			}
			m.mu.Unlock()

			m.logger.Info("connected to rabbitmq successfully")
			go m.reconnectLoop(conn.NotifyClose(make(chan *amqp.Error, 1)))
			return
		}

		m.logger.Warn("broker connection attempt failed, retrying...", "error", err.Error(), "next_backoff", backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

// reconnectLoop listens for connection closure and attempts reconnection with exponential backoff.
func (m *Manager) reconnectLoop(closeErr chan *amqp.Error) {
	err, ok := <-closeErr
	if !ok {
		// Clean close
		return
	}

	m.mu.Lock()
	if m.closed {
		m.mu.Unlock()
		return
	}
	m.pubCh = nil
	m.conn = nil
	m.reconnecting = true

	// Replace notifyReady channel so new callers can wait for reconnection
	select {
	case <-m.notifyReady:
		m.notifyReady = make(chan struct{})
	default:
	}
	m.mu.Unlock()

	m.logger.Warn("rabbitmq connection closed, reconnecting...", "reason", err.Error())

	m.startRetryLoop()
}

// Publish sends a persistent, mandatory message and waits for publisher confirm and return.
func (m *Manager) Publish(ctx context.Context, exchange, routingKey string, msg amqp.Publishing) error {
	m.mu.RLock()
	pubCh := m.pubCh
	confirms := m.confirms
	returns := m.returns
	m.mu.RUnlock()

	if pubCh == nil {
		return ErrNotConnected
	}

	// Always ensure persistent delivery mode
	msg.DeliveryMode = amqp.Persistent

	// Publish with mandatory=true to detect unroutable messages
	if err := pubCh.PublishWithContext(
		ctx,
		exchange,
		routingKey,
		true,  // mandatory
		false, // immediate
		msg,
	); err != nil {
		return fmt.Errorf("failed to publish amqp message: %w", err)
	}

	// Wait for confirm or return
	select {
	case <-ctx.Done():
		return ctx.Err()
	case ret, ok := <-returns:
		if ok && ret.MessageId == msg.MessageId {
			return fmt.Errorf("%w: reply_code=%d reply_text=%s exchange=%s routing_key=%s",
				ErrUnroutable, ret.ReplyCode, ret.ReplyText, ret.Exchange, ret.RoutingKey)
		}
	case conf, ok := <-confirms:
		if !ok {
			return ErrNotConnected
		}
		if !conf.Ack {
			return ErrPublishNack
		}
		return nil
	}

	return nil
}

// OpenConsumerChannel opens a new channel with QoS prefetch configured.
func (m *Manager) OpenConsumerChannel(prefetch int) (*amqp.Channel, error) {
	m.mu.RLock()
	conn := m.conn
	m.mu.RUnlock()

	if conn == nil || conn.IsClosed() {
		return nil, ErrNotConnected
	}

	ch, err := conn.Channel()
	if err != nil {
		return nil, fmt.Errorf("failed to open consumer channel: %w", err)
	}

	if err := ch.Qos(prefetch, 0, false); err != nil {
		_ = ch.Close()
		return nil, fmt.Errorf("failed to configure qos prefetch %d: %w", prefetch, err)
	}

	return ch, nil
}

// WaitForConnection blocks until an active connection is established, the context expires,
// or the manager is closed.
func (m *Manager) WaitForConnection(ctx context.Context) error {
	m.mu.RLock()
	if m.closed {
		m.mu.RUnlock()
		return errors.New("amqp broker manager is closed")
	}
	if m.conn != nil && !m.conn.IsClosed() {
		m.mu.RUnlock()
		return nil
	}
	ready := m.notifyReady
	m.mu.RUnlock()

	select {
	case <-ready:
		m.mu.RLock()
		defer m.mu.RUnlock()
		if m.closed {
			return errors.New("amqp broker manager is closed")
		}
		if m.conn != nil && !m.conn.IsClosed() {
			return nil
		}
		return ErrNotConnected
	case <-ctx.Done():
		return ctx.Err()
	}
}

// CheckHealth returns nil if connection is active, or an error if disconnected.
func (m *Manager) CheckHealth(_ context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return errors.New("amqp broker manager is closed")
	}
	if m.conn == nil || m.conn.IsClosed() {
		return ErrNotConnected
	}
	return nil
}

// Close gracefully closes the publisher channel and connection.
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.closed = true

	// Unblock any waiters on notifyReady
	select {
	case <-m.notifyReady:
	default:
		close(m.notifyReady)
	}

	var errs []error
	if m.pubCh != nil {
		if err := m.pubCh.Close(); err != nil && err != amqp.ErrClosed {
			errs = append(errs, err)
		}
		m.pubCh = nil
	}

	if m.conn != nil {
		if err := m.conn.Close(); err != nil && err != amqp.ErrClosed {
			errs = append(errs, err)
		}
		m.conn = nil
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors closing broker manager: %v", errs)
	}
	return nil
}
