// Package provider defines the messaging provider interface and registry.
package provider

import "context"

// TopicManager is an optional interface that providers can implement
// to support topic/queue management operations.
//
// Not all providers need to implement this interface. Clients can check
// if a provider supports topic management using type assertion:
//
//	if tm, ok := provider.(TopicManager); ok {
//		// Provider supports topic management
//	}
type TopicManager interface {
	// CreateTopic creates a new topic/queue with the specified configuration.
	//
	// For Kafka: Creates a topic with partitions and replication factor
	// For RabbitMQ: Declares an exchange and/or queue
	//
	// Returns an error if the topic already exists or creation fails.
	CreateTopic(ctx context.Context, config *TopicConfig) error

	// DeleteTopic removes a topic/queue.
	//
	// For Kafka: Deletes the topic
	// For RabbitMQ: Deletes the queue and optionally the exchange
	//
	// Returns an error if the topic doesn't exist or deletion fails.
	DeleteTopic(ctx context.Context, topic string) error

	// TopicExists checks if a topic/queue exists.
	//
	// Returns true if the topic exists, false otherwise.
	TopicExists(ctx context.Context, topic string) (bool, error)

	// ListTopics returns a list of all topics/queues.
	//
	// For Kafka: Returns all topics
	// For RabbitMQ: Returns all queues
	//
	// The returned list may be filtered based on provider capabilities.
	ListTopics(ctx context.Context) ([]string, error)

	// UpdateTopicConfig updates the configuration of an existing topic.
	//
	// For Kafka: Updates topic configuration (retention, etc.)
	// For RabbitMQ: May not be supported for all properties
	//
	// Returns an error if the topic doesn't exist or update fails.
	UpdateTopicConfig(ctx context.Context, topic string, config *TopicConfig) error
}

// TopicConfig contains configuration for creating/updating a topic.
//
// Different providers use different subsets of these fields:
//   - Kafka: Uses Partitions, ReplicationFactor, ConfigEntries
//   - RabbitMQ: Uses ExchangeType, Durable, AutoDelete, Bindings
type TopicConfig struct {
	// Name is the topic/queue name (required)
	Name string

	// Partitions specifies the number of partitions (Kafka only)
	Partitions int32

	// ReplicationFactor specifies the replication factor (Kafka only)
	ReplicationFactor int16

	// ConfigEntries contains provider-specific configuration
	// For Kafka: retention.ms, cleanup.policy, etc.
	ConfigEntries map[string]string

	// ExchangeType specifies the exchange type (RabbitMQ only)
	// Valid values: "direct", "topic", "fanout", "headers"
	ExchangeType string

	// Durable indicates if the topic/queue should survive broker restart
	Durable bool

	// AutoDelete indicates if the topic/queue should be deleted when no longer used
	AutoDelete bool

	// Bindings contains routing key bindings (RabbitMQ only)
	// Maps routing keys to exchange names
	Bindings map[string]string

	// Arguments contains additional provider-specific arguments
	Arguments map[string]interface{}
}

// Validate checks if the topic configuration is valid.
func (tc *TopicConfig) Validate() error {
	if tc.Name == "" {
		return ErrInvalidTopicName
	}

	// Kafka validations
	if tc.Partitions < 0 {
		return ErrInvalidPartitionCount
	}

	if tc.ReplicationFactor < 0 {
		return ErrInvalidReplicationFactor
	}

	// RabbitMQ validations
	if tc.ExchangeType != "" {
		validTypes := map[string]bool{
			"direct":  true,
			"topic":   true,
			"fanout":  true,
			"headers": true,
		}
		if !validTypes[tc.ExchangeType] {
			return ErrInvalidExchangeType
		}
	}

	return nil
}

// TopicInfo contains information about a topic/queue.
type TopicInfo struct {
	// Name is the topic/queue name
	Name string

	// Partitions is the number of partitions (Kafka only)
	Partitions int32

	// ReplicationFactor is the replication factor (Kafka only)
	ReplicationFactor int16

	// Config contains provider-specific configuration
	Config map[string]string

	// MessageCount is the approximate number of messages (if available)
	MessageCount int64

	// ConsumerCount is the number of active consumers (if available)
	ConsumerCount int
}
