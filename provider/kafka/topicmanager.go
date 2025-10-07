package kafka

import (
	"context"
	"fmt"
	"log"

	"github.com/IBM/sarama"
	"github.com/ciaranRoche/heimdall-go/provider"
)

// Ensure Provider implements TopicManager interface
var _ provider.TopicManager = (*Provider)(nil)

// CreateTopic creates a new Kafka topic with the specified configuration.
func (p *Provider) CreateTopic(ctx context.Context, config *provider.TopicConfig) error {
	if err := config.Validate(); err != nil {
		return fmt.Errorf("invalid topic configuration: %w", err)
	}

	admin, err := p.getClusterAdmin()
	if err != nil {
		return fmt.Errorf("failed to create cluster admin: %w", err)
	}
	defer func() {
		_ = admin.Close() // Best effort close
	}()

	// Check if topic already exists
	exists, err := p.TopicExists(ctx, config.Name)
	if err != nil {
		return fmt.Errorf("failed to check if topic exists: %w", err)
	}
	if exists {
		return provider.ErrTopicAlreadyExists
	}

	// Set default values if not specified
	partitions := config.Partitions
	if partitions == 0 {
		partitions = 1 // Default to 1 partition
	}

	replicationFactor := config.ReplicationFactor
	if replicationFactor == 0 {
		replicationFactor = 1 // Default to replication factor of 1
	}

	// Build topic detail
	topicDetail := &sarama.TopicDetail{
		NumPartitions:     partitions,
		ReplicationFactor: replicationFactor,
		ConfigEntries:     make(map[string]*string),
	}

	// Add config entries
	for key, value := range config.ConfigEntries {
		v := value // Create new variable for pointer
		topicDetail.ConfigEntries[key] = &v
	}

	// Create topic
	err = admin.CreateTopic(config.Name, topicDetail, false)
	if err != nil {
		return fmt.Errorf("failed to create topic %s: %w", config.Name, err)
	}

	log.Printf("Kafka TopicManager: Created topic %s (partitions=%d, replication=%d)",
		config.Name, partitions, replicationFactor)

	return nil
}

// DeleteTopic removes a Kafka topic.
func (p *Provider) DeleteTopic(ctx context.Context, topic string) error {
	if topic == "" {
		return provider.ErrInvalidTopicName
	}

	admin, err := p.getClusterAdmin()
	if err != nil {
		return fmt.Errorf("failed to create cluster admin: %w", err)
	}
	defer func() {
		_ = admin.Close() // Best effort close
	}()

	// Check if topic exists
	exists, err := p.TopicExists(ctx, topic)
	if err != nil {
		return fmt.Errorf("failed to check if topic exists: %w", err)
	}
	if !exists {
		return provider.ErrTopicNotFound
	}

	// Delete topic
	err = admin.DeleteTopic(topic)
	if err != nil {
		return fmt.Errorf("failed to delete topic %s: %w", topic, err)
	}

	log.Printf("Kafka TopicManager: Deleted topic %s", topic)

	return nil
}

// TopicExists checks if a Kafka topic exists.
func (p *Provider) TopicExists(ctx context.Context, topic string) (bool, error) {
	if topic == "" {
		return false, provider.ErrInvalidTopicName
	}

	admin, err := p.getClusterAdmin()
	if err != nil {
		return false, fmt.Errorf("failed to create cluster admin: %w", err)
	}
	defer func() {
		_ = admin.Close() // Best effort close
	}()

	// List all topics
	topics, err := admin.ListTopics()
	if err != nil {
		return false, fmt.Errorf("failed to list topics: %w", err)
	}

	_, exists := topics[topic]
	return exists, nil
}

// ListTopics returns a list of all Kafka topics.
func (p *Provider) ListTopics(ctx context.Context) ([]string, error) {
	admin, err := p.getClusterAdmin()
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster admin: %w", err)
	}
	defer func() {
		_ = admin.Close() // Best effort close
	}()

	// List all topics
	topicsMap, err := admin.ListTopics()
	if err != nil {
		return nil, fmt.Errorf("failed to list topics: %w", err)
	}

	// Convert map to slice
	topics := make([]string, 0, len(topicsMap))
	for topic := range topicsMap {
		// Skip internal Kafka topics
		if topic != "__consumer_offsets" && topic != "__transaction_state" {
			topics = append(topics, topic)
		}
	}

	return topics, nil
}

// UpdateTopicConfig updates the configuration of an existing Kafka topic.
func (p *Provider) UpdateTopicConfig(ctx context.Context, topic string, config *provider.TopicConfig) error {
	if topic == "" {
		return provider.ErrInvalidTopicName
	}

	admin, err := p.getClusterAdmin()
	if err != nil {
		return fmt.Errorf("failed to create cluster admin: %w", err)
	}
	defer func() {
		_ = admin.Close() // Best effort close
	}()

	// Check if topic exists
	exists, err := p.TopicExists(ctx, topic)
	if err != nil {
		return fmt.Errorf("failed to check if topic exists: %w", err)
	}
	if !exists {
		return provider.ErrTopicNotFound
	}

	// Build config entries for update
	configEntries := make(map[string]*string)
	for key, value := range config.ConfigEntries {
		v := value // Create new variable for pointer
		configEntries[key] = &v
	}

	// Alter topic config
	err = admin.AlterConfig(sarama.TopicResource, topic, configEntries, false)
	if err != nil {
		return fmt.Errorf("failed to update topic config for %s: %w", topic, err)
	}

	log.Printf("Kafka TopicManager: Updated config for topic %s", topic)

	return nil
}

// GetTopicInfo retrieves detailed information about a Kafka topic.
func (p *Provider) GetTopicInfo(ctx context.Context, topic string) (*provider.TopicInfo, error) {
	if topic == "" {
		return nil, provider.ErrInvalidTopicName
	}

	admin, err := p.getClusterAdmin()
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster admin: %w", err)
	}
	defer func() {
		_ = admin.Close() // Best effort close
	}()

	// Get topic metadata
	metadata, err := admin.DescribeTopics([]string{topic})
	if err != nil {
		return nil, fmt.Errorf("failed to describe topic %s: %w", topic, err)
	}

	if len(metadata) == 0 {
		return nil, provider.ErrTopicNotFound
	}

	topicMeta := metadata[0]
	if topicMeta.Err != sarama.ErrNoError {
		return nil, fmt.Errorf("topic error: %w", topicMeta.Err)
	}

	// Get topic config
	configResource, err := admin.DescribeConfig(sarama.ConfigResource{
		Type: sarama.TopicResource,
		Name: topic,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe topic config: %w", err)
	}

	configMap := make(map[string]string)
	for _, entry := range configResource {
		configMap[entry.Name] = entry.Value
	}

	// Calculate replication factor from first partition
	replicationFactor := int16(0)
	if len(topicMeta.Partitions) > 0 {
		// #nosec G115 - Kafka replication factor is limited by cluster size, typically < 100
		replicationFactor = int16(len(topicMeta.Partitions[0].Replicas))
	}

	info := &provider.TopicInfo{
		Name: topic,
		// #nosec G115 - Kafka partition count is limited by configuration, typically < 10000
		Partitions:        int32(len(topicMeta.Partitions)),
		ReplicationFactor: replicationFactor,
		Config:            configMap,
	}

	return info, nil
}

// getClusterAdmin creates a Sarama cluster admin client.
func (p *Provider) getClusterAdmin() (sarama.ClusterAdmin, error) {
	// Extract bootstrap servers
	bootstrapServers, err := extractBootstrapServers(p.config)
	if err != nil {
		return nil, err
	}

	// Create Sarama config
	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Version = sarama.V3_6_0_0

	// Apply security configuration
	applySecurity(kafkaConfig, p.config)

	// Create cluster admin
	admin, err := sarama.NewClusterAdmin(bootstrapServers, kafkaConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster admin: %w", err)
	}

	return admin, nil
}
