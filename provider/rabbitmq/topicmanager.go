package rabbitmq

import (
	"context"
	"fmt"
	"log"

	"github.com/ciaranRoche/heimdall-go/provider"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Ensure Provider implements TopicManager interface
var _ provider.TopicManager = (*Provider)(nil)

// CreateTopic creates a new RabbitMQ queue (and optionally an exchange).
//
// For RabbitMQ, a "topic" can be either a queue, an exchange, or both.
// The behavior depends on the configuration:
//   - If ExchangeType is specified, an exchange is created
//   - A queue is always created and bound to the exchange
//
// nolint:gocyclo // Complexity justified by comprehensive RabbitMQ topology setup
func (p *Provider) CreateTopic(ctx context.Context, config *provider.TopicConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid topic configuration: %w", err)
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	durable, autoDelete := p.getDurabilitySettings(config)

	// Create exchange if exchange type is specified
	if config.ExchangeType != "" {
		if err := p.createExchange(config, durable, autoDelete); err != nil {
			return err
		}
	}

	// Create queue
	queue, err := p.createQueue(config, durable, autoDelete)
	if err != nil {
		return err
	}

	log.Printf("RabbitMQ TopicManager: Created queue %s (durable=%v, messages=%d)",
		queue.Name, durable, queue.Messages)

	// Bind queue to exchange if bindings are specified
	if len(config.Bindings) > 0 {
		if err := p.bindQueueToExchanges(queue.Name, config.Bindings); err != nil {
			return err
		}
	}

	return nil
}

// getDurabilitySettings extracts durability settings from config
func (p *Provider) getDurabilitySettings(config *provider.TopicConfig) (durable bool, autoDelete bool) {
	durable = config.Durable
	if !durable && config.ConfigEntries != nil {
		if val, ok := config.ConfigEntries["durable"]; ok && val == "true" {
			durable = true
		}
	}
	autoDelete = config.AutoDelete
	return durable, autoDelete
}

// createExchange creates a RabbitMQ exchange
func (p *Provider) createExchange(config *provider.TopicConfig, durable, autoDelete bool) error {
	exchangeName := config.Name
	if config.Arguments != nil {
		if name, ok := config.Arguments["exchange_name"].(string); ok {
			exchangeName = name
		}
	}

	// Check if exchange already exists
	err := p.channel.ExchangeDeclarePassive(
		exchangeName,
		config.ExchangeType,
		durable,
		autoDelete,
		false, // internal
		false, // no-wait
		nil,   // args
	)

	if err == nil {
		// Exchange already exists
		return provider.ErrTopicAlreadyExists
	}

	// Create the exchange
	args := amqp.Table{}
	if config.Arguments != nil {
		for k, v := range config.Arguments {
			if k != "exchange_name" { // Skip our internal arg
				args[k] = v
			}
		}
	}

	err = p.channel.ExchangeDeclare(
		exchangeName,
		config.ExchangeType,
		durable,
		autoDelete,
		false, // internal
		false, // no-wait
		args,
	)
	if err != nil {
		return fmt.Errorf("failed to create exchange %s: %w", exchangeName, err)
	}

	log.Printf("RabbitMQ TopicManager: Created exchange %s (type=%s, durable=%v)",
		exchangeName, config.ExchangeType, durable)

	return nil
}

// createQueue creates a RabbitMQ queue
func (p *Provider) createQueue(config *provider.TopicConfig, durable, autoDelete bool) (amqp.Queue, error) {
	args := amqp.Table{}
	if config.Arguments != nil {
		for k, v := range config.Arguments {
			if k != "exchange_name" {
				args[k] = v
			}
		}
	}

	// Add config entries to arguments
	for k, v := range config.ConfigEntries {
		if k != "durable" && k != "auto_delete" {
			args[k] = v
		}
	}

	queue, err := p.channel.QueueDeclare(
		config.Name,
		durable,
		autoDelete,
		false, // exclusive
		false, // no-wait
		args,
	)
	if err != nil {
		return amqp.Queue{}, fmt.Errorf("failed to create queue %s: %w", config.Name, err)
	}

	return queue, nil
}

// bindQueueToExchanges binds a queue to multiple exchanges
func (p *Provider) bindQueueToExchanges(queueName string, bindings map[string]string) error {
	for routingKey, exchangeName := range bindings {
		err := p.channel.QueueBind(
			queueName,
			routingKey,
			exchangeName,
			false, // no-wait
			nil,   // args
		)
		if err != nil {
			return fmt.Errorf("failed to bind queue %s to exchange %s with key %s: %w",
				queueName, exchangeName, routingKey, err)
		}

		log.Printf("RabbitMQ TopicManager: Bound queue %s to exchange %s (routing key: %s)",
			queueName, exchangeName, routingKey)
	}
	return nil
}

// DeleteTopic removes a RabbitMQ queue.
//
// Note: This only deletes the queue, not exchanges.
// To delete an exchange, use DeleteExchange method.
func (p *Provider) DeleteTopic(ctx context.Context, topic string) error {
	if topic == "" {
		return provider.ErrInvalidTopicName
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Try to delete queue (if exists)
	_, err := p.channel.QueueDelete(
		topic,
		false, // if-unused
		false, // if-empty
		false, // no-wait
	)

	if err != nil {
		return fmt.Errorf("failed to delete queue %s: %w", topic, err)
	}

	log.Printf("RabbitMQ TopicManager: Deleted queue %s", topic)

	return nil
}

// DeleteExchange removes a RabbitMQ exchange.
func (p *Provider) DeleteExchange(ctx context.Context, exchange string) error {
	if exchange == "" {
		return provider.ErrInvalidTopicName
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	err := p.channel.ExchangeDelete(
		exchange,
		false, // if-unused
		false, // no-wait
	)

	if err != nil {
		return fmt.Errorf("failed to delete exchange %s: %w", exchange, err)
	}

	log.Printf("RabbitMQ TopicManager: Deleted exchange %s", exchange)

	return nil
}

// TopicExists checks if a RabbitMQ queue exists.
//
// Note: RabbitMQ closes the channel when QueueDeclarePassive() is called on a non-existent queue.
// To handle this, we need to reopen the channel if it gets closed.
func (p *Provider) TopicExists(ctx context.Context, topic string) (bool, error) {
	if topic == "" {
		return false, provider.ErrInvalidTopicName
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Try passive queue declare
	_, err := p.channel.QueueDeclarePassive(
		topic,
		false, // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)

	if err != nil {
		// Queue doesn't exist - RabbitMQ closes the channel in this case
		// We need to reopen the channel for subsequent operations
		if err := p.reopenChannel(); err != nil {
			return false, fmt.Errorf("failed to reopen channel after passive declare: %w", err)
		}
		return false, nil
	}

	return true, nil
}

// reopenChannel reopens the RabbitMQ channel after it has been closed.
// This is necessary because RabbitMQ closes the channel on certain errors like
// QueueDeclarePassive on non-existent queues.
func (p *Provider) reopenChannel() error {
	// Check if connection is still valid
	if p.connection == nil || p.connection.IsClosed() {
		return fmt.Errorf("connection is closed, cannot reopen channel")
	}

	// Close old channel if it's not nil (best effort)
	if p.channel != nil {
		_ = p.channel.Close()
	}

	// Open new channel
	ch, err := p.connection.Channel()
	if err != nil {
		return fmt.Errorf("failed to open new channel: %w", err)
	}

	// Get exchange configuration from original config
	exchange := getStringConfig(p.config, "exchange", "heimdall.events")
	exchangeType := getStringConfig(p.config, "exchange_type", "topic")
	durable := getBoolConfig(p.config, "durable", true)

	// Re-declare exchange on new channel
	err = ch.ExchangeDeclare(
		exchange,
		exchangeType,
		durable,
		false, // auto-deleted
		false, // internal
		false, // no-wait
		nil,   // arguments
	)
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to re-declare exchange: %w", err)
	}

	// Reset QoS on new channel
	prefetchCount := getIntConfig(p.config, "prefetch_count", 1)
	err = ch.Qos(
		prefetchCount,
		0,
		false,
	)
	if err != nil {
		_ = ch.Close()
		return fmt.Errorf("failed to set QoS on new channel: %w", err)
	}

	// Update provider's channel
	p.channel = ch
	log.Printf("RabbitMQ Provider: Reopened channel successfully")

	return nil
}

// ExchangeExists checks if a RabbitMQ exchange exists.
func (p *Provider) ExchangeExists(ctx context.Context, exchange string, exchangeType string) (bool, error) {
	if exchange == "" {
		return false, provider.ErrInvalidTopicName
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Try passive exchange declare
	err := p.channel.ExchangeDeclarePassive(
		exchange,
		exchangeType,
		false, // durable
		false, // auto-delete
		false, // internal
		false, // no-wait
		nil,   // args
	)

	if err != nil {
		// Exchange doesn't exist - RabbitMQ closes the channel in this case
		// We need to reopen the channel for subsequent operations
		if err := p.reopenChannel(); err != nil {
			return false, fmt.Errorf("failed to reopen channel after passive declare: %w", err)
		}
		return false, nil
	}

	return true, nil
}

// ListTopics returns a list of all RabbitMQ queues.
//
// Note: RabbitMQ doesn't provide a native API to list queues via AMQP protocol.
// This would require using the RabbitMQ Management HTTP API.
// This implementation returns an error indicating the limitation.
func (p *Provider) ListTopics(ctx context.Context) ([]string, error) {
	return nil, fmt.Errorf("listing queues requires RabbitMQ Management HTTP API, not supported via AMQP")
}

// UpdateTopicConfig updates the configuration of an existing RabbitMQ queue.
//
// Note: RabbitMQ doesn't support modifying queue properties after creation.
// The queue must be deleted and recreated with new properties.
// This method returns an error indicating the limitation.
func (p *Provider) UpdateTopicConfig(ctx context.Context, topic string, config *provider.TopicConfig) error {
	return fmt.Errorf("updating queue properties not supported in RabbitMQ; delete and recreate the queue instead")
}

// GetTopicInfo retrieves information about a RabbitMQ queue.
//
// Returns queue information including message count and consumer count.
func (p *Provider) GetTopicInfo(ctx context.Context, topic string) (*provider.TopicInfo, error) {
	if topic == "" {
		return nil, provider.ErrInvalidTopicName
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// Use passive queue declare to inspect queue
	queue, err := p.channel.QueueDeclarePassive(
		topic,
		false, // durable
		false, // auto-delete
		false, // exclusive
		false, // no-wait
		nil,   // args
	)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect queue %s: %w", topic, err)
	}

	info := &provider.TopicInfo{
		Name:          queue.Name,
		MessageCount:  int64(queue.Messages),
		ConsumerCount: queue.Consumers,
		Config:        make(map[string]string),
	}

	// Note: RabbitMQ doesn't expose all queue properties via AMQP inspect
	// For full details, would need Management HTTP API

	return info, nil
}

// BindQueue binds a queue to an exchange with a routing key.
func (p *Provider) BindQueue(ctx context.Context, queueName, exchangeName, routingKey string) error {
	if queueName == "" || exchangeName == "" {
		return fmt.Errorf("queue name and exchange name are required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	err := p.channel.QueueBind(
		queueName,
		routingKey,
		exchangeName,
		false, // no-wait
		nil,   // args
	)

	if err != nil {
		return fmt.Errorf("failed to bind queue %s to exchange %s: %w", queueName, exchangeName, err)
	}

	log.Printf("RabbitMQ TopicManager: Bound queue %s to exchange %s (routing key: %s)",
		queueName, exchangeName, routingKey)

	return nil
}

// UnbindQueue removes a binding between a queue and an exchange.
func (p *Provider) UnbindQueue(ctx context.Context, queueName, exchangeName, routingKey string) error {
	if queueName == "" || exchangeName == "" {
		return fmt.Errorf("queue name and exchange name are required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	err := p.channel.QueueUnbind(
		queueName,
		routingKey,
		exchangeName,
		nil, // args
	)

	if err != nil {
		return fmt.Errorf("failed to unbind queue %s from exchange %s: %w", queueName, exchangeName, err)
	}

	log.Printf("RabbitMQ TopicManager: Unbound queue %s from exchange %s (routing key: %s)",
		queueName, exchangeName, routingKey)

	return nil
}
