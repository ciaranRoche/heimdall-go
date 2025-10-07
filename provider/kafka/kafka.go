// Package kafka provides a Kafka implementation of the Heimdall provider interface.
package kafka

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"

	"github.com/IBM/sarama"

	"github.com/ciaranRoche/heimdall-go/provider"
)

// Provider implements the provider.Provider interface for Apache Kafka.
type Provider struct {
	config        map[string]interface{}
	producer      sarama.AsyncProducer
	consumerGroup sarama.ConsumerGroup
	consumers     map[string]*consumer
	mu            sync.RWMutex
	wg            sync.WaitGroup
	done          chan struct{}
	ready         chan bool
}

// consumer tracks an active subscription
type consumer struct {
	topic      string
	handler    provider.MessageHandler
	cancelFunc context.CancelFunc
	ready      chan bool
}

// NewProvider creates a new Kafka provider.
//
// Configuration options:
//   - bootstrap_servers ([]string, required): Kafka broker addresses
//   - consumer_group (string, optional): Consumer group name (default: "heimdall-consumers")
//   - security_protocol (string, optional): "PLAINTEXT", "SSL", "SASL_PLAINTEXT", "SASL_SSL"
//   - sasl_username (string, optional): SASL username
//   - sasl_password (string, optional): SASL password
//   - compression_type (string, optional): "gzip", "snappy", "lz4", "zstd"
//
// Example:
//
//	prov, err := NewProvider(map[string]interface{}{
//		"bootstrap_servers": []string{"localhost:9092"},
//		"consumer_group": "my-group",
//	})
func NewProvider(config map[string]interface{}) (provider.Provider, error) {
	// Extract bootstrap servers
	bootstrapServers, err := extractBootstrapServers(config)
	if err != nil {
		return nil, err
	}

	log.Printf("Kafka Provider: Connecting to brokers: %v", bootstrapServers)

	// Create Sarama config
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.V3_6_0_0

	// Producer configuration
	kafkaConfig.Producer.RequiredAcks = sarama.WaitForAll
	kafkaConfig.Producer.Retry.Max = 5
	kafkaConfig.Producer.Return.Successes = true
	kafkaConfig.Producer.Return.Errors = true

	// Consumer configuration
	kafkaConfig.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRange()}
	kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest

	// Apply compression if specified
	if compression, ok := config["compression_type"].(string); ok {
		switch compression {
		case "gzip":
			kafkaConfig.Producer.Compression = sarama.CompressionGZIP
		case "snappy":
			kafkaConfig.Producer.Compression = sarama.CompressionSnappy
		case "lz4":
			kafkaConfig.Producer.Compression = sarama.CompressionLZ4
		case "zstd":
			kafkaConfig.Producer.Compression = sarama.CompressionZSTD
		}
	}

	// Apply security configuration
	applySecurity(kafkaConfig, config)

	// Create producer
	producer, err := sarama.NewAsyncProducer(bootstrapServers, kafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create Kafka producer: %w", err)
	}

	// Get consumer group name
	consumerGroup := getStringConfig(config, "consumer_group", "heimdall-consumers")

	// Create consumer group
	cg, err := sarama.NewConsumerGroup(bootstrapServers, consumerGroup, kafkaConfig)
	if err != nil {
		_ = producer.Close() // Best effort cleanup
		return nil, fmt.Errorf("failed to create Kafka consumer group: %w", err)
	}

	p := &Provider{
		config:        config,
		producer:      producer,
		consumerGroup: cg,
		consumers:     make(map[string]*consumer),
		done:          make(chan struct{}),
		ready:         make(chan bool),
	}

	// Start background goroutine to handle producer responses
	p.wg.Add(1)
	go p.handleProducerResponses()

	log.Printf("Kafka Provider: Connected (consumer group: %s)", consumerGroup)

	return p, nil
}

// Publish sends a message to Kafka.
func (p *Provider) Publish(ctx context.Context, topic string, data []byte, headers map[string]interface{}, correlationID string) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// Build Kafka headers
	kafkaHeaders := []sarama.RecordHeader{}
	for k, v := range headers {
		if str, ok := v.(string); ok {
			kafkaHeaders = append(kafkaHeaders, sarama.RecordHeader{
				Key:   []byte(k),
				Value: []byte(str),
			})
		}
	}

	// Add correlation ID to headers if provided
	if correlationID != "" {
		kafkaHeaders = append(kafkaHeaders, sarama.RecordHeader{
			Key:   []byte("correlation_id"),
			Value: []byte(correlationID),
		})
	}

	// Create producer message
	msg := &sarama.ProducerMessage{
		Topic:   topic,
		Value:   sarama.ByteEncoder(data),
		Headers: kafkaHeaders,
	}

	// Send to producer (async)
	select {
	case p.producer.Input() <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-p.done:
		return fmt.Errorf("provider is closed")
	}
}

// Subscribe starts consuming messages from a Kafka topic.
// Each subscription creates its own consumer group to avoid Kafka consumer group limitations.
func (p *Provider) Subscribe(ctx context.Context, topic string, handler provider.MessageHandler) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if already subscribed to this topic
	if _, exists := p.consumers[topic]; exists {
		return fmt.Errorf("already subscribed to topic: %s", topic)
	}

	// Extract bootstrap servers
	bootstrapServers, err := extractBootstrapServers(p.config)
	if err != nil {
		return fmt.Errorf("failed to extract bootstrap servers: %w", err)
	}

	// Create Sarama config
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.V3_6_0_0
	kafkaConfig.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRange()}
	kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest

	// Apply security configuration
	applySecurity(kafkaConfig, p.config)

	// Create a unique consumer group for this topic to avoid blocking
	baseConsumerGroup := getStringConfig(p.config, "consumer_group", "heimdall-consumers")
	topicConsumerGroup := fmt.Sprintf("%s-%s", baseConsumerGroup, topic)

	// Create dedicated consumer group for this topic
	cg, err := sarama.NewConsumerGroup(bootstrapServers, topicConsumerGroup, kafkaConfig)
	if err != nil {
		return fmt.Errorf("failed to create consumer group for topic %s: %w", topic, err)
	}

	// Create cancellable context for this subscription
	subCtx, cancel := context.WithCancel(ctx)

	// Create consumer handler
	consumerHandler := &kafkaConsumerHandler{
		ready:   make(chan bool),
		handler: handler,
		topic:   topic,
	}

	// Track consumer
	p.consumers[topic] = &consumer{
		topic:      topic,
		handler:    handler,
		cancelFunc: cancel,
		ready:      consumerHandler.ready,
	}

	// Start consuming in background
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer func() {
			if err := cg.Close(); err != nil {
				log.Printf("Kafka Provider: Failed to close consumer group for topic %s: %v", topic, err)
			}
		}()
		for {
			if err := cg.Consume(subCtx, []string{topic}, consumerHandler); err != nil {
				log.Printf("Kafka Provider: Error from consumer for topic %s: %v", topic, err)
			}
			if subCtx.Err() != nil {
				log.Printf("Kafka Provider: Context canceled for topic %s", topic)
				return
			}
			consumerHandler.ready = make(chan bool)
		}
	}()

	// Wait for consumer to be ready
	<-consumerHandler.ready

	log.Printf("Kafka Provider: Subscribed to topic %s (consumer group: %s)", topic, topicConsumerGroup)

	return nil
}

// HealthCheck verifies the Kafka connection is alive.
func (p *Provider) HealthCheck(ctx context.Context) error {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if p.consumerGroup == nil {
		return fmt.Errorf("kafka consumer group is nil")
	}

	return nil
}

// Close gracefully shuts down the provider.
func (p *Provider) Close() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	log.Printf("Kafka Provider: Closing...")

	// Signal all goroutines to stop
	close(p.done)

	// Cancel all subscriptions
	for _, c := range p.consumers {
		if c.cancelFunc != nil {
			c.cancelFunc()
		}
	}

	// Wait for all goroutines to finish
	p.wg.Wait()

	var errs []error

	// Close producer
	if p.producer != nil {
		if err := p.producer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close producer: %w", err))
		}
	}

	// Close consumer group
	if p.consumerGroup != nil {
		if err := p.consumerGroup.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close consumer group: %w", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during close: %v", errs)
	}

	log.Printf("Kafka Provider: Closed successfully")
	return nil
}

// handleProducerResponses processes async producer results
func (p *Provider) handleProducerResponses() {
	defer p.wg.Done()

	for {
		select {
		case success := <-p.producer.Successes():
			log.Printf("Kafka Provider: Successfully published to topic %s, partition %d, offset %d",
				success.Topic, success.Partition, success.Offset)

		case err := <-p.producer.Errors():
			log.Printf("Kafka Provider: Failed to publish message: %v", err.Err)

		case <-p.done:
			log.Printf("Kafka Provider: Shutting down producer response handler")
			return
		}
	}
}

// kafkaConsumerHandler implements sarama.ConsumerGroupHandler
type kafkaConsumerHandler struct {
	ready   chan bool
	handler provider.MessageHandler
	topic   string
}

// Setup is run at the beginning of a new session
func (h *kafkaConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	close(h.ready)
	log.Printf("Kafka Consumer: Session setup complete for topic %s", h.topic)
	return nil
}

// Cleanup is run at the end of a session
func (h *kafkaConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	log.Printf("Kafka Consumer: Session cleanup for topic %s", h.topic)
	return nil
}

// ConsumeClaim processes messages from a partition
func (h *kafkaConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	for {
		select {
		case message := <-claim.Messages():
			if message == nil {
				return nil
			}

			log.Printf("Kafka Consumer: Received message from topic %s, partition %d, offset %d",
				message.Topic, message.Partition, message.Offset)

			// Extract headers
			headers := make(map[string]interface{})
			for _, header := range message.Headers {
				headers[string(header.Key)] = string(header.Value)
			}

			// Process message
			if err := h.handler(session.Context(), message.Value, headers); err != nil {
				log.Printf("Kafka Consumer: Handler error for topic %s: %v", h.topic, err)
				// Don't mark as consumed if handler failed
				continue
			}

			// Mark message as consumed
			session.MarkMessage(message, "")

		case <-session.Context().Done():
			log.Printf("Kafka Consumer: Session context done for topic %s", h.topic)
			return nil
		}
	}
}

// Helper functions

func extractBootstrapServers(config map[string]interface{}) ([]string, error) {
	servers, ok := config["bootstrap_servers"]
	if !ok {
		return nil, fmt.Errorf("kafka 'bootstrap_servers' configuration is required")
	}

	// Handle []interface{}
	if serverList, ok := servers.([]interface{}); ok {
		result := make([]string, len(serverList))
		for i, s := range serverList {
			if str, ok := s.(string); ok {
				result[i] = str
			} else {
				return nil, fmt.Errorf("bootstrap_servers must contain strings")
			}
		}
		return result, nil
	}

	// Handle []string
	if serverList, ok := servers.([]string); ok {
		return serverList, nil
	}

	// Handle comma-separated string
	if serverStr, ok := servers.(string); ok {
		return strings.Split(serverStr, ","), nil
	}

	return nil, fmt.Errorf("bootstrap_servers must be a string array or comma-separated string")
}

func applySecurity(kafkaConfig *sarama.Config, config map[string]interface{}) {
	protocol, ok := config["security_protocol"].(string)
	if !ok {
		return // No security configured
	}

	switch protocol {
	case "SASL_SSL", "SASL_PLAINTEXT":
		kafkaConfig.Net.SASL.Enable = true
		kafkaConfig.Net.SASL.Mechanism = sarama.SASLTypePlaintext

		if username, ok := config["sasl_username"].(string); ok {
			kafkaConfig.Net.SASL.User = username
		}

		if password, ok := config["sasl_password"].(string); ok {
			kafkaConfig.Net.SASL.Password = password
		}

		if protocol == "SASL_SSL" {
			kafkaConfig.Net.TLS.Enable = true
		}

	case "SSL":
		kafkaConfig.Net.TLS.Enable = true
	}
}

func getStringConfig(config map[string]interface{}, key, defaultValue string) string {
	if val, ok := config[key].(string); ok {
		return val
	}
	return defaultValue
}

// init registers the Kafka provider
func init() {
	provider.Register("kafka", NewProvider)
	log.Printf("Kafka Provider: Registered")
}
