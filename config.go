package heimdall

import (
	"fmt"
)

// Config holds the configuration for a Heimdall instance.
type Config struct {
	// Provider is the messaging provider to use (e.g., "kafka", "rabbitmq")
	Provider string

	// Config contains provider-specific configuration
	Config map[string]interface{}

	// ConsumerGroup is the consumer group name (for providers that support it)
	ConsumerGroup string

	// Observability configuration
	Observability *ObservabilityConfig

	// Routing configuration
	Routing *RoutingConfig
}

// ObservabilityConfig configures metrics, tracing, and logging.
type ObservabilityConfig struct {
	// Metrics enables metrics collection
	Metrics bool

	// Tracing enables distributed tracing
	Tracing bool

	// Logger is a custom logger implementation
	Logger Logger

	// ServiceName is the service name used in traces and metrics
	ServiceName string

	// ServiceVersion is the service version used in traces and metrics
	ServiceVersion string
}

// RoutingConfig configures message routing behavior.
type RoutingConfig struct {
	// Routes defines routing rules
	Routes []Route

	// DefaultTopic is the fallback topic if no routes match
	DefaultTopic string
}

// Route defines a routing rule.
type Route struct {
	// Pattern is a pattern to match against (e.g., "order.*", "user.created")
	Pattern string

	// Target is the destination topic/queue
	Target string

	// ResponseHandler is an optional handler for response messages
	ResponseHandler MessageHandler

	// ResponseTopic is the topic to listen for responses on
	ResponseTopic string
}

// Logger is an interface for structured logging.
// Implementations can wrap zap, zerolog, logrus, or the standard logger.
type Logger interface {
	Debug(msg string, fields ...interface{})
	Info(msg string, fields ...interface{})
	Warn(msg string, fields ...interface{})
	Error(msg string, fields ...interface{})
}

// Validate checks if the configuration is valid.
func (c *Config) Validate() error {
	if c.Provider == "" {
		return fmt.Errorf("provider must be specified")
	}

	if c.Config == nil {
		c.Config = make(map[string]interface{})
	}

	return nil
}

// Option is a functional option for configuring Heimdall.
type Option func(*Config) error

// WithProvider sets the messaging provider.
//
// Example:
//
//	heimdall.New(heimdall.WithProvider("kafka"))
func WithProvider(provider string) Option {
	return func(c *Config) error {
		c.Provider = provider
		return nil
	}
}

// WithConfig sets provider-specific configuration.
//
// Example:
//
//	heimdall.New(
//		heimdall.WithProvider("kafka"),
//		heimdall.WithConfig(map[string]interface{}{
//			"bootstrap_servers": []string{"localhost:9092"},
//			"consumer_group": "my-group",
//		}),
//	)
func WithConfig(config map[string]interface{}) Option {
	return func(c *Config) error {
		c.Config = config
		return nil
	}
}

// WithConsumerGroup sets the consumer group name.
//
// This is a convenience option for providers that support consumer groups.
// Alternatively, you can set this in the provider configuration directly.
func WithConsumerGroup(group string) Option {
	return func(c *Config) error {
		c.ConsumerGroup = group
		if c.Config == nil {
			c.Config = make(map[string]interface{})
		}
		c.Config["consumer_group"] = group
		return nil
	}
}

// WithObservability enables observability features.
//
// Example:
//
//	heimdall.New(
//		heimdall.WithProvider("kafka"),
//		heimdall.WithObservability(&heimdall.ObservabilityConfig{
//			Metrics: true,
//			Tracing: true,
//			ServiceName: "my-service",
//		}),
//	)
func WithObservability(cfg *ObservabilityConfig) Option {
	return func(c *Config) error {
		c.Observability = cfg
		return nil
	}
}

// WithRouting configures message routing.
//
// Example:
//
//	heimdall.New(
//		heimdall.WithProvider("kafka"),
//		heimdall.WithRouting(&heimdall.RoutingConfig{
//			Routes: []heimdall.Route{
//				{
//					Pattern: "order.*",
//					Target: "orders.topic",
//				},
//			},
//		}),
//	)
func WithRouting(cfg *RoutingConfig) Option {
	return func(c *Config) error {
		c.Routing = cfg
		return nil
	}
}

// PublishOptions contains options for publishing messages.
type PublishOptions struct {
	// Headers are key-value pairs attached to the message
	Headers map[string]interface{}

	// CorrelationID is an optional correlation identifier
	CorrelationID string

	// Key is an optional partition key (for Kafka)
	Key string
}

// PublishOption is a functional option for Publish operations.
type PublishOption func(*PublishOptions)

// WithHeaders sets message headers.
//
// Example:
//
//	h.Publish(ctx, "topic", data,
//		heimdall.WithHeaders(map[string]interface{}{
//			"content-type": "application/json",
//			"user-id": "123",
//		}),
//	)
func WithHeaders(headers map[string]interface{}) PublishOption {
	return func(o *PublishOptions) {
		o.Headers = headers
	}
}

// WithCorrelationID sets a correlation ID for request/response tracking.
//
// Example:
//
//	h.Publish(ctx, "topic", data,
//		heimdall.WithCorrelationID("req-123"),
//	)
func WithCorrelationID(id string) PublishOption {
	return func(o *PublishOptions) {
		o.CorrelationID = id
	}
}

// WithKey sets the partition key for Kafka messages.
//
// Example:
//
//	h.Publish(ctx, "topic", data,
//		heimdall.WithKey("user-123"),
//	)
func WithKey(key string) PublishOption {
	return func(o *PublishOptions) {
		o.Key = key
	}
}
