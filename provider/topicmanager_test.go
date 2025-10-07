package provider

import (
	"testing"
)

func TestTopicConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *TopicConfig
		wantErr error
	}{
		{
			name: "valid_kafka_config",
			config: &TopicConfig{
				Name:              "test-topic",
				Partitions:        3,
				ReplicationFactor: 2,
				ConfigEntries: map[string]string{
					"retention.ms": "86400000",
				},
			},
			wantErr: nil,
		},
		{
			name: "valid_rabbitmq_config",
			config: &TopicConfig{
				Name:         "test-queue",
				ExchangeType: "topic",
				Durable:      true,
				Bindings: map[string]string{
					"test.#": "test-exchange",
				},
			},
			wantErr: nil,
		},
		{
			name: "empty_name",
			config: &TopicConfig{
				Name:       "",
				Partitions: 1,
			},
			wantErr: ErrInvalidTopicName,
		},
		{
			name: "negative_partitions",
			config: &TopicConfig{
				Name:       "test-topic",
				Partitions: -1,
			},
			wantErr: ErrInvalidPartitionCount,
		},
		{
			name: "negative_replication_factor",
			config: &TopicConfig{
				Name:              "test-topic",
				Partitions:        1,
				ReplicationFactor: -1,
			},
			wantErr: ErrInvalidReplicationFactor,
		},
		{
			name: "invalid_exchange_type",
			config: &TopicConfig{
				Name:         "test-queue",
				ExchangeType: "invalid",
			},
			wantErr: ErrInvalidExchangeType,
		},
		{
			name: "valid_direct_exchange",
			config: &TopicConfig{
				Name:         "test-queue",
				ExchangeType: "direct",
			},
			wantErr: nil,
		},
		{
			name: "valid_fanout_exchange",
			config: &TopicConfig{
				Name:         "test-queue",
				ExchangeType: "fanout",
			},
			wantErr: nil,
		},
		{
			name: "valid_headers_exchange",
			config: &TopicConfig{
				Name:         "test-queue",
				ExchangeType: "headers",
			},
			wantErr: nil,
		},
		{
			name: "zero_values_valid",
			config: &TopicConfig{
				Name:              "test-topic",
				Partitions:        0, // Will use defaults
				ReplicationFactor: 0, // Will use defaults
			},
			wantErr: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if err != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTopicInfo(t *testing.T) {
	info := &TopicInfo{
		Name:              "test-topic",
		Partitions:        3,
		ReplicationFactor: 2,
		Config: map[string]string{
			"retention.ms": "86400000",
		},
		MessageCount:  1000,
		ConsumerCount: 2,
	}

	if info.Name != "test-topic" {
		t.Errorf("Expected name 'test-topic', got '%s'", info.Name)
	}

	if info.Partitions != 3 {
		t.Errorf("Expected 3 partitions, got %d", info.Partitions)
	}

	if info.ReplicationFactor != 2 {
		t.Errorf("Expected replication factor 2, got %d", info.ReplicationFactor)
	}

	if info.MessageCount != 1000 {
		t.Errorf("Expected 1000 messages, got %d", info.MessageCount)
	}

	if info.ConsumerCount != 2 {
		t.Errorf("Expected 2 consumers, got %d", info.ConsumerCount)
	}

	if retention, ok := info.Config["retention.ms"]; !ok || retention != "86400000" {
		t.Errorf("Expected retention.ms='86400000', got '%s'", retention)
	}
}
