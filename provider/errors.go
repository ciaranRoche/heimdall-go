package provider

import "errors"

// Topic management errors
var (
	// ErrInvalidTopicName is returned when the topic name is empty or invalid
	ErrInvalidTopicName = errors.New("invalid topic name: name cannot be empty")

	// ErrInvalidPartitionCount is returned when partition count is invalid
	ErrInvalidPartitionCount = errors.New("invalid partition count: must be >= 0")

	// ErrInvalidReplicationFactor is returned when replication factor is invalid
	ErrInvalidReplicationFactor = errors.New("invalid replication factor: must be >= 0")

	// ErrInvalidExchangeType is returned when exchange type is invalid
	ErrInvalidExchangeType = errors.New("invalid exchange type: must be direct, topic, fanout, or headers")

	// ErrTopicAlreadyExists is returned when trying to create an existing topic
	ErrTopicAlreadyExists = errors.New("topic already exists")

	// ErrTopicNotFound is returned when topic doesn't exist
	ErrTopicNotFound = errors.New("topic not found")

	// ErrTopicManagementNotSupported is returned when provider doesn't support topic management
	ErrTopicManagementNotSupported = errors.New("topic management not supported by this provider")
)
