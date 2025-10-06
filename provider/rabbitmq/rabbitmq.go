// Package rabbitmq provides a RabbitMQ implementation of the Heimdall provider interface.
package rabbitmq

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"

	"github.com/ciaranRoche/heimdall-go/provider"
)

// Provider implements the provider.Provider interface for RabbitMQ.
type Provider struct {
	config     map[string]interface{}
	connection *amqp.Connection
	channel    *amqp.Channel
	exchange   string
	mu         sync.RWMutex
	done       chan bool
	wg         sync.WaitGroup
	consumers  map[string]*consumer
}

// consumer tracks an active subscription
type consumer struct {
	topic      string
	handler    provider.MessageHandler
	cancelFunc context.CancelFunc
}

// NewProvider creates a new RabbitMQ provider.
//
// Configuration options:
//   - url (string, required): RabbitMQ connection URL (e.g., "amqp://user:pass@localhost:5672/")
//   - exchange (string, optional): Exchange name (default: "heimdall.events")
//   - exchange_type (string, optional): Exchange type (default: "topic")
//   - queue_name (string, optional): Queue name for consumers (default: auto-generated)
//   - durable (bool, optional): Make queues/exchanges durable (default: true)
//   - prefetch_count (int, optional): QoS prefetch count (default: 1)
//   - auto_ack (bool, optional): Auto-acknowledge messages (default: false)
//
// Example:
//
//	prov, err := NewProvider(map[string]interface{}{
//		"url": "amqp://guest:guest@localhost:5672/",
//		"exchange": "my-exchange",
//		"exchange_type": "topic",
//	})
func NewProvider(config map[string]interface{}) (provider.Provider, error) {
	// Extract connection URL
	url, ok := config["url"].(string)
	if !ok {
		return nil, fmt.Errorf("rabbitmq 'url' configuration is required")
	}

	// Connect to RabbitMQ
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to RabbitMQ: %w", err)
	}

	// Create channel
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close() // Best effort cleanup
		return nil, fmt.Errorf("failed to open channel: %w", err)
	}

	// Get exchange configuration
	exchange := getStringConfig(config, "exchange", "heimdall.events")
	exchangeType := getStringConfig(config, "exchange_type", "topic")
	durable := getBoolConfig(config, "durable", true)

	// Declare exchange
	err = ch.ExchangeDeclare(
		exchange,
		exchangeType,
		durable, // durable
		false,   // auto-deleted
		false,   // internal
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		_ = ch.Close()   // Best effort cleanup
		_ = conn.Close() // Best effort cleanup
		return nil, fmt.Errorf("failed to declare exchange: %w", err)
	}

	// Set QoS
	prefetchCount := getIntConfig(config, "prefetch_count", 1)
	err = ch.Qos(
		prefetchCount, // prefetch count
		0,             // prefetch size
		false,         // global
	)
	if err != nil {
		_ = ch.Close()   // Best effort cleanup
		_ = conn.Close() // Best effort cleanup
		return nil, fmt.Errorf("failed to set QoS: %w", err)
	}

	log.Printf("RabbitMQ Provider: Connected to RabbitMQ (exchange: %s, type: %s)", exchange, exchangeType)

	return &Provider{
		config:     config,
		connection: conn,
		channel:    ch,
		exchange:   exchange,
		done:       make(chan bool),
		consumers:  make(map[string]*consumer),
	}, nil
}

// Publish sends a message to RabbitMQ.
func (p *Provider) Publish(ctx context.Context, topic string, data []byte, headers map[string]interface{}, correlationID string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Build AMQP headers
	amqpHeaders := amqp.Table{}
	for k, v := range headers {
		amqpHeaders[k] = v
	}

	// Create publishing
	publishing := amqp.Publishing{
		ContentType:   "application/json",
		CorrelationId: correlationID,
		Headers:       amqpHeaders,
		Body:          data,
		Timestamp:     time.Now(),
		DeliveryMode:  amqp.Persistent, // Persistent delivery
	}

	// Publish to exchange with routing key
	err := p.channel.PublishWithContext(
		ctx,
		p.exchange, // exchange
		topic,      // routing key
		false,      // mandatory
		false,      // immediate
		publishing,
	)

	if err != nil {
		return fmt.Errorf("failed to publish message: %w", err)
	}

	return nil
}

// Subscribe starts consuming messages from a queue bound to the topic/routing pattern.
func (p *Provider) Subscribe(ctx context.Context, topic string, handler provider.MessageHandler) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if already subscribed to this topic
	if _, exists := p.consumers[topic]; exists {
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}

	// Get queue name from config or generate one
	queueName := getStringConfig(p.config, "queue_name", "")
	if queueName == "" {
		queueName = fmt.Sprintf("heimdall.%s.%d", topic, time.Now().UnixNano())
	}

	durable := getBoolConfig(p.config, "durable", true)

	// Declare queue
	queue, err := p.channel.QueueDeclare(
		queueName,
		durable, // durable
		false,   // delete when unused
		false,   // exclusive
		false,   // no-wait
		nil,     // arguments
	)
	if err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	// Bind queue to exchange with routing pattern
	err = p.channel.QueueBind(
		queue.Name,
		topic, // routing key/pattern
		p.exchange,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to bind queue: %w", err)
	}

	autoAck := getBoolConfig(p.config, "auto_ack", false)

	// Start consuming
	msgs, err := p.channel.Consume(
		queue.Name,
		"",      // consumer tag (auto-generated)
		autoAck, // auto-ack
		false,   // exclusive
		false,   // no-local
		false,   // no-wait
		nil,     // args
	)
	if err != nil {
		return fmt.Errorf("failed to start consumer: %w", err)
	}

	// Create cancellable context for this subscription
	subCtx, cancel := context.WithCancel(ctx)

	// Track consumer
	p.consumers[topic] = &consumer{
		topic:      topic,
		handler:    handler,
		cancelFunc: cancel,
	}

	// Start message processing goroutine
	p.wg.Add(1)
	go p.processMessages(subCtx, topic, msgs, handler, autoAck)

	log.Printf("RabbitMQ Provider: Subscribed to topic %s (queue: %s)", topic, queue.Name)

	return nil
}

// processMessages handles incoming messages for a subscription.
func (p *Provider) processMessages(ctx context.Context, topic string, msgs <-chan amqp.Delivery, handler provider.MessageHandler, autoAck bool) {
	defer p.wg.Done()

	log.Printf("RabbitMQ Provider: Started processing messages for topic: %s", topic)

	for {
		select {
		case msg, ok := <-msgs:
			if !ok {
				log.Printf("RabbitMQ Provider: Message channel closed for topic: %s", topic)
				return
			}

			// Convert AMQP headers to map
			headers := make(map[string]interface{})
			for k, v := range msg.Headers {
				headers[k] = v
			}

			// Add message metadata
			headers["correlation_id"] = msg.CorrelationId
			headers["reply_to"] = msg.ReplyTo
			headers["message_id"] = msg.MessageId
			headers["timestamp"] = msg.Timestamp

			// Process message
			err := handler(ctx, msg.Body, headers)
			if err != nil {
				log.Printf("RabbitMQ Provider: Handler error for topic %s: %v", topic, err)

				// Reject message if not auto-ack
				if !autoAck {
					if rejectErr := msg.Reject(false); rejectErr != nil {
						log.Printf("RabbitMQ Provider: Failed to reject message: %v", rejectErr)
					}
				}
			} else {
				// Acknowledge message if not auto-ack
				if !autoAck {
					if ackErr := msg.Ack(false); ackErr != nil {
						log.Printf("RabbitMQ Provider: Failed to ack message: %v", ackErr)
					}
				}
			}

		case <-ctx.Done():
			log.Printf("RabbitMQ Provider: Context canceled for topic: %s", topic)
			return

		case <-p.done:
			log.Printf("RabbitMQ Provider: Stop signal received for topic: %s", topic)
			return
		}
	}
}

// HealthCheck verifies the RabbitMQ connection is alive.
func (p *Provider) HealthCheck(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.connection == nil || p.connection.IsClosed() {
		return fmt.Errorf("rabbitmq connection is closed")
	}

	if p.channel == nil {
		return fmt.Errorf("rabbitmq channel is not available")
	}

	return nil
}

// Close gracefully shuts down the provider.
func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Printf("RabbitMQ Provider: Closing...")

	// Signal all consumers to stop
	close(p.done)

	// Cancel all subscriptions
	for _, c := range p.consumers {
		if c.cancelFunc != nil {
			c.cancelFunc()
		}
	}

	// Wait for all message processors to finish
	p.wg.Wait()

	var errs []error

	// Close channel
	if p.channel != nil {
		if err := p.channel.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close channel: %w", err))
		}
	}

	// Close connection
	if p.connection != nil {
		if err := p.connection.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close connection: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	log.Printf("RabbitMQ Provider: Closed successfully")
	return nil
}

// Helper functions for config parsing
func getStringConfig(config map[string]interface{}, key, defaultValue string) string {
	if val, ok := config[key].(string); ok {
		return val
	}
	return defaultValue
}

func getIntConfig(config map[string]interface{}, key string, defaultValue int) int {
	if val, ok := config[key].(int); ok {
		return val
	}
	return defaultValue
}

func getBoolConfig(config map[string]interface{}, key string, defaultValue bool) bool {
	if val, ok := config[key].(bool); ok {
		return val
	}
	return defaultValue
}

// init registers the RabbitMQ provider
func init() {
	provider.Register("rabbitmq", NewProvider)
	log.Printf("RabbitMQ Provider: Registered")
}
