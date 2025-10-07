// Package heimdall provides a unified messaging abstraction layer for Apache Kafka,
// RabbitMQ, and other messaging providers.
//
// Heimdall enables applications to publish and consume messages across different
// messaging systems through a consistent interface, with built-in support for routing,
// observability, and provider-agnostic patterns.
//
// Basic usage:
//
//	h, err := heimdall.New(
//		heimdall.WithProvider("kafka"),
//		heimdall.WithConfig(map[string]interface{}{
//			"bootstrap_servers": []string{"localhost:9092"},
//		}),
//	)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer h.Close()
//
//	// Publish a message
//	ctx := context.Background()
//	err = h.Publish(ctx, "my-topic", []byte("Hello, Heimdall!"))
package heimdall

import (
	"context"
	"fmt"
	"sync"

	"github.com/ciaranRoche/heimdall-go/provider"
)

// Heimdall is the main entry point for the messaging abstraction layer.
// It provides a unified interface for publishing and consuming messages
// across different messaging providers.
type Heimdall struct {
	provider  provider.Provider
	config    *Config
	mu        sync.RWMutex
	consumers map[string]context.CancelFunc
	closed    bool
}

// Message represents a message received from a messaging provider.
type Message struct {
	// Topic is the topic/queue the message was received from
	Topic string

	// Data is the raw message payload
	Data []byte

	// Headers contains message metadata
	Headers map[string]interface{}

	// CorrelationID is an optional correlation identifier
	CorrelationID string

	// Provider-specific metadata
	Metadata map[string]interface{}
}

// MessageHandler is a function that processes received messages.
// It should return an error if the message cannot be processed,
// which will trigger provider-specific retry/error handling.
type MessageHandler func(ctx context.Context, msg *Message) error

// New creates a new Heimdall instance with the provided options.
//
// At minimum, you must specify a provider using WithProvider() and
// provide configuration using WithConfig().
//
// Example:
//
//	h, err := heimdall.New(
//		heimdall.WithProvider("kafka"),
//		heimdall.WithConfig(map[string]interface{}{
//			"bootstrap_servers": []string{"localhost:9092"},
//			"consumer_group": "my-group",
//		}),
//	)
func New(opts ...Option) (*Heimdall, error) {
	cfg := &Config{
		Provider: "kafka",
		Config:   make(map[string]interface{}),
	}

	for _, opt := range opts {
		if err := opt(cfg); err != nil {
			return nil, fmt.Errorf("failed to apply option: %w", err)
		}
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Create provider
	prov, err := provider.NewProvider(cfg.Provider, cfg.Config)
	if err != nil {
		return nil, fmt.Errorf("failed to create provider: %w", err)
	}

	h := &Heimdall{
		provider:  prov,
		config:    cfg,
		consumers: make(map[string]context.CancelFunc),
	}

	return h, nil
}

// Publish sends a message to the specified topic/queue.
//
// The ctx parameter can be used to set deadlines and cancellation.
// The topic parameter is provider-specific (Kafka topic, RabbitMQ routing key, etc.).
// The data parameter contains the message payload.
//
// Example:
//
//	err := h.Publish(ctx, "orders.created", []byte(`{"order_id": "123"}`))
func (h *Heimdall) Publish(ctx context.Context, topic string, data []byte, opts ...PublishOption) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return fmt.Errorf("heimdall instance is closed")
	}

	pubOpts := &PublishOptions{}
	for _, opt := range opts {
		opt(pubOpts)
	}

	return h.provider.Publish(ctx, topic, data, pubOpts.Headers, pubOpts.CorrelationID)
}

// Subscribe starts consuming messages from the specified topic/queue.
//
// The handler function will be called for each message received.
// Subscription runs in a background goroutine and can be canceled via the
// returned cancel function or by calling Close() on the Heimdall instance.
//
// Example:
//
//	cancel, err := h.Subscribe(ctx, "orders.created", func(ctx context.Context, msg *heimdall.Message) error {
//		log.Printf("Received: %s", msg.Data)
//		return nil
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer cancel()
func (h *Heimdall) Subscribe(ctx context.Context, topic string, handler MessageHandler) (context.CancelFunc, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil, fmt.Errorf("heimdall instance is closed")
	}

	// Create cancellable context for this subscription
	subCtx, cancel := context.WithCancel(ctx)

	// Wrap the handler to convert provider messages to Heimdall messages
	providerHandler := func(ctx context.Context, data []byte, headers map[string]interface{}) error {
		msg := &Message{
			Topic:         topic,
			Data:          data,
			Headers:       headers,
			CorrelationID: getCorrelationID(headers),
			Metadata:      make(map[string]interface{}),
		}
		return handler(ctx, msg)
	}

	// Start consuming
	if err := h.provider.Subscribe(subCtx, topic, providerHandler); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
	}

	// Track the subscription
	h.consumers[topic] = cancel

	return cancel, nil
}

// HealthCheck performs a health check on the underlying messaging provider.
//
// This can be used in Kubernetes readiness/liveness probes or application
// health endpoints.
func (h *Heimdall) HealthCheck(ctx context.Context) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return fmt.Errorf("heimdall instance is closed")
	}

	return h.provider.HealthCheck(ctx)
}

// Close gracefully shuts down the Heimdall instance, stopping all consumers
// and closing connections to the messaging provider.
//
// This should be called before application shutdown, typically via defer:
//
//	h, err := heimdall.New(...)
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer h.Close()
func (h *Heimdall) Close() error {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.closed {
		return nil
	}

	h.closed = true

	// Cancel all subscriptions
	for _, cancel := range h.consumers {
		cancel()
	}

	// Close provider
	if err := h.provider.Close(); err != nil {
		return fmt.Errorf("failed to close provider: %w", err)
	}

	return nil
}

// CreateTopic creates a new topic/queue with the specified configuration.
//
// This operation requires the underlying provider to support topic management.
// Not all providers support this operation - check provider documentation.
//
// Returns provider.ErrTopicManagementNotSupported if the provider doesn't support topic management.
func (h *Heimdall) CreateTopic(ctx context.Context, config *provider.TopicConfig) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return fmt.Errorf("heimdall instance is closed")
	}

	tm, ok := h.provider.(provider.TopicManager)
	if !ok {
		return provider.ErrTopicManagementNotSupported
	}

	return tm.CreateTopic(ctx, config)
}

// DeleteTopic removes a topic/queue.
//
// This operation requires the underlying provider to support topic management.
// Not all providers support this operation - check provider documentation.
//
// Returns provider.ErrTopicManagementNotSupported if the provider doesn't support topic management.
func (h *Heimdall) DeleteTopic(ctx context.Context, topic string) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return fmt.Errorf("heimdall instance is closed")
	}

	tm, ok := h.provider.(provider.TopicManager)
	if !ok {
		return provider.ErrTopicManagementNotSupported
	}

	return tm.DeleteTopic(ctx, topic)
}

// TopicExists checks if a topic/queue exists.
//
// This operation requires the underlying provider to support topic management.
// Not all providers support this operation - check provider documentation.
//
// Returns provider.ErrTopicManagementNotSupported if the provider doesn't support topic management.
func (h *Heimdall) TopicExists(ctx context.Context, topic string) (bool, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return false, fmt.Errorf("heimdall instance is closed")
	}

	tm, ok := h.provider.(provider.TopicManager)
	if !ok {
		return false, provider.ErrTopicManagementNotSupported
	}

	return tm.TopicExists(ctx, topic)
}

// ListTopics returns a list of all topics/queues.
//
// This operation requires the underlying provider to support topic management.
// Not all providers support this operation - check provider documentation.
//
// Returns provider.ErrTopicManagementNotSupported if the provider doesn't support topic management.
func (h *Heimdall) ListTopics(ctx context.Context) ([]string, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return nil, fmt.Errorf("heimdall instance is closed")
	}

	tm, ok := h.provider.(provider.TopicManager)
	if !ok {
		return nil, provider.ErrTopicManagementNotSupported
	}

	return tm.ListTopics(ctx)
}

// UpdateTopicConfig updates the configuration of an existing topic/queue.
//
// This operation requires the underlying provider to support topic management.
// Not all providers support this operation - check provider documentation.
//
// Returns provider.ErrTopicManagementNotSupported if the provider doesn't support topic management.
func (h *Heimdall) UpdateTopicConfig(ctx context.Context, topic string, config *provider.TopicConfig) error {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.closed {
		return fmt.Errorf("heimdall instance is closed")
	}

	tm, ok := h.provider.(provider.TopicManager)
	if !ok {
		return provider.ErrTopicManagementNotSupported
	}

	return tm.UpdateTopicConfig(ctx, topic, config)
}

// SupportsTopicManagement returns true if the underlying provider supports topic management.
func (h *Heimdall) SupportsTopicManagement() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, ok := h.provider.(provider.TopicManager)
	return ok
}

// getCorrelationID extracts correlation ID from message headers
func getCorrelationID(headers map[string]interface{}) string {
	if headers == nil {
		return ""
	}

	// Try common header names
	for _, key := range []string{"correlation_id", "correlationId", "x-correlation-id"} {
		if val, ok := headers[key]; ok {
			if str, ok := val.(string); ok {
				return str
			}
		}
	}

	return ""
}
