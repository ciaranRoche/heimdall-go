// Package provider defines the messaging provider interface and registry.
//
// Providers implement the core messaging abstraction for different systems
// (Kafka, RabbitMQ, etc.). New providers can be registered via the Register
// function, typically in an init() function.
package provider

import (
	"context"
	"fmt"
	"sync"
)

// Provider is the core interface that all messaging providers must implement.
//
// Implementations should handle provider-specific connection management,
// message serialization, and error handling.
type Provider interface {
	// Publish sends a message to the specified topic/queue.
	//
	// Parameters:
	//   - ctx: Context for cancellation and deadlines
	//   - topic: The destination topic/queue name
	//   - data: The message payload
	//   - headers: Optional message headers/metadata
	//   - correlationID: Optional correlation identifier
	Publish(ctx context.Context, topic string, data []byte, headers map[string]interface{}, correlationID string) error

	// Subscribe starts consuming messages from a topic/queue.
	//
	// The handler function will be called for each message received.
	// Subscription should run in a background goroutine and respect
	// context cancellation.
	//
	// Parameters:
	//   - ctx: Context for cancellation (subscription stops when canceled)
	//   - topic: The source topic/queue name
	//   - handler: Function to process each message
	Subscribe(ctx context.Context, topic string, handler MessageHandler) error

	// HealthCheck verifies the connection to the messaging system.
	//
	// This should perform a lightweight check (e.g., ping, list topics)
	// and return an error if the system is unavailable.
	HealthCheck(ctx context.Context) error

	// Close gracefully shuts down the provider, closing connections
	// and stopping all consumers.
	Close() error
}

// MessageHandler processes messages received from a provider.
//
// Parameters:
//   - ctx: Context for the message processing
//   - data: The message payload
//   - headers: Message headers/metadata
//
// If the handler returns an error, the provider should handle it according
// to its semantics (e.g., retry, send to DLQ, etc.).
type MessageHandler func(ctx context.Context, data []byte, headers map[string]interface{}) error

// Factory is a function that creates a new provider instance.
type Factory func(config map[string]interface{}) (Provider, error)

var (
	registry = make(map[string]Factory)
	mu       sync.RWMutex
)

// Register registers a provider factory with the given name.
//
// This function is typically called from init() functions in provider
// implementations:
//
//	func init() {
//		provider.Register("kafka", NewKafkaProvider)
//	}
//
// If a provider with the same name already exists, it will be overwritten.
func Register(name string, factory Factory) {
	mu.Lock()
	defer mu.Unlock()
	registry[name] = factory
}

// NewProvider creates a new provider instance by name.
//
// The provider must have been previously registered via Register().
//
// Example:
//
//	prov, err := provider.NewProvider("kafka", map[string]interface{}{
//		"bootstrap_servers": []string{"localhost:9092"},
//	})
func NewProvider(name string, config map[string]interface{}) (Provider, error) {
	mu.RLock()
	factory, exists := registry[name]
	mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("provider '%s' not found (did you import the provider package?)", name)
	}

	return factory(config)
}

// ListProviders returns a list of all registered provider names.
//
// This is useful for debugging and displaying available providers.
func ListProviders() []string {
	mu.RLock()
	defer mu.RUnlock()

	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	return names
}

// IsRegistered checks if a provider with the given name is registered.
func IsRegistered(name string) bool {
	mu.RLock()
	defer mu.RUnlock()
	_, exists := registry[name]
	return exists
}
