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
func (m *Manager) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.conn != nil && !m.conn.IsClosed() {
		return nil
	}

	return m.connectLocked(ctx)
}

func (m *Manager) connectLocked(_ context.Context) error {
	conn, err := amqp.Dial(m.url)
	if err != nil {
		return fmt.Errorf("failed to dial rabbitmq: %w", err)
	}

	pubCh, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("failed to open publisher channel: %w", err)
	}

	// 1. Declare Topology idempotently
	if err := DeclareTopology(pubCh); err != nil {
		_ = pubCh.Close()
		_ = conn.Close()
		return fmt.Errorf("failed to declare broker topology: %w", err)
	}

	// 2. Put publisher channel into confirm mode
	if err := pubCh.Confirm(false); err != nil {
		_ = pubCh.Close()
		_ = conn.Close()
		return fmt.Errorf("failed to put channel into confirm mode: %w", err)
	}

	confirms := pubCh.NotifyPublish(make(chan amqp.Confirmation, 100))
	returns := pubCh.NotifyReturn(make(chan amqp.Return, 100))

	m.conn = conn
	m.pubCh = pubCh
	m.confirms = confirms
	m.returns = returns

	m.logger.Info("connected to rabbitmq and declared topology")

	// Monitor connection loss in background
	go m.reconnectLoop(conn.NotifyClose(make(chan *amqp.Error, 1)))

	return nil
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
	m.mu.Unlock()

	m.logger.Warn("rabbitmq connection closed, reconnecting...", "reason", err.Error())

	backoff := 500 * time.Millisecond
	maxBackoff := 10 * time.Second

	for {
		m.mu.RLock()
		isClosed := m.closed
		m.mu.RUnlock()
		if isClosed {
			return
		}

		time.Sleep(backoff)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		m.mu.Lock()
		connErr := m.connectLocked(ctx)
		if connErr == nil {
			m.reconnecting = false
			m.mu.Unlock()
			cancel()
			m.logger.Info("reconnected to rabbitmq successfully")
			return
		}
		m.mu.Unlock()
		cancel()

		m.logger.Warn("reconnect attempt failed, retrying...", "error", connErr.Error(), "next_backoff", backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
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

// CheckHealth returns nil if connection is active, or an error if disconnected.
func (m *Manager) CheckHealth(ctx context.Context) error {
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
