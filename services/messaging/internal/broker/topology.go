package broker

import (
	"fmt"

	amqp "github.com/rabbitmq/amqp091-go"
)

const (
	// TopicExchange is the authoritative primary events exchange.
	TopicExchange = "swiftdrop.events.topic"

	// DLQExchange is the fanout dead-letter exchange.
	DLQExchange = "swiftdrop.dlq.exchange"

	// DLQueue is the dead-letter queue where failed messages are routed.
	DLQueue = "swiftdrop.dlq"

	// DLQMessageTTL is the message retention TTL on DLQ in milliseconds (30 days).
	DLQMessageTTL int64 = 2592000000

	// QueueNotificationsWorker processes customer email notifications.
	QueueNotificationsWorker = "q.notifications.worker"

	// QueueWebhooksDispatcher processes outbound merchant webhook delivery.
	QueueWebhooksDispatcher = "q.webhooks.dispatcher"

	// QueueTrackingUpdates provides real-time event projection for Tracking API (Member 05).
	QueueTrackingUpdates = "q.tracking.updates"
)

// QueueDefinition defines a queue and its topic routing key bindings.
type QueueDefinition struct {
	Name        string
	RoutingKeys []string
}

// WorkQueues returns the list of all worker queues and their routing key bindings.
func WorkQueues() []QueueDefinition {
	return []QueueDefinition{
		{
			Name:        QueueNotificationsWorker,
			RoutingKeys: []string{"parcel.#"},
		},
		{
			Name:        QueueWebhooksDispatcher,
			RoutingKeys: []string{"parcel.#", "courier.#", "merchant.#"},
		},
		{
			Name:        QueueTrackingUpdates,
			RoutingKeys: []string{"parcel.#", "courier.#"},
		},
	}
}

// DeclareTopology idempotently declares all exchanges, queues, and bindings for SwiftDrop.
func DeclareTopology(ch *amqp.Channel) error {
	// 1. Declare DLX Exchange (Fanout, Durable)
	if err := ch.ExchangeDeclare(
		DLQExchange,
		amqp.ExchangeFanout,
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	); err != nil {
		return fmt.Errorf("failed to declare DLX exchange %s: %w", DLQExchange, err)
	}

	// 2. Declare DLQ Queue with message TTL (30 days)
	dlqArgs := amqp.Table{
		"x-message-ttl": DLQMessageTTL,
	}
	if _, err := ch.QueueDeclare(
		DLQueue,
		true,  // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		dlqArgs,
	); err != nil {
		return fmt.Errorf("failed to declare DLQ queue %s: %w", DLQueue, err)
	}

	// 3. Bind DLQ to DLX
	if err := ch.QueueBind(
		DLQueue,
		"", // fanout routing key
		DLQExchange,
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("failed to bind DLQ %s to DLX %s: %w", DLQueue, DLQExchange, err)
	}

	// 4. Declare Primary Topic Exchange (Topic, Durable)
	if err := ch.ExchangeDeclare(
		TopicExchange,
		amqp.ExchangeTopic,
		true,  // durable
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare topic exchange %s: %w", TopicExchange, err)
	}

	// 5. Declare Work Queues with DLX configuration and topic bindings
	for _, qDef := range WorkQueues() {
		qArgs := amqp.Table{
			"x-dead-letter-exchange": DLQExchange,
		}
		if _, err := ch.QueueDeclare(
			qDef.Name,
			true,  // durable
			false, // auto-delete
			false, // exclusive
			false, // no-wait
			qArgs,
		); err != nil {
			return fmt.Errorf("failed to declare queue %s: %w", qDef.Name, err)
		}

		for _, rk := range qDef.RoutingKeys {
			if err := ch.QueueBind(
				qDef.Name,
				rk,
				TopicExchange,
				false, // no-wait
				nil,
			); err != nil {
				return fmt.Errorf("failed to bind queue %s to %s with routing key %s: %w", qDef.Name, TopicExchange, rk, err)
			}
		}
	}

	return nil
}
