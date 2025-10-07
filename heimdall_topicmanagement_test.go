package heimdall

import (
	"context"
	"testing"

	"github.com/ciaranRoche/heimdall-go/provider"
)

// mockProviderWithTopicManagement implements both Provider and TopicManager
type mockProviderWithTopicManagement struct {
	mockProvider
	createCalled       bool
	deleteCalled       bool
	existsCalled       bool
	listCalled         bool
	updateCalled       bool
	topics             map[string]*provider.TopicConfig
	shouldReturnExists bool
}

func (m *mockProviderWithTopicManagement) CreateTopic(ctx context.Context, config *provider.TopicConfig) error {
	m.createCalled = true
	if m.topics == nil {
		m.topics = make(map[string]*provider.TopicConfig)
	}
	if _, exists := m.topics[config.Name]; exists {
		return provider.ErrTopicAlreadyExists
	}
	m.topics[config.Name] = config
	return nil
}

func (m *mockProviderWithTopicManagement) DeleteTopic(ctx context.Context, topic string) error {
	m.deleteCalled = true
	if _, exists := m.topics[topic]; !exists {
		return provider.ErrTopicNotFound
	}
	delete(m.topics, topic)
	return nil
}

func (m *mockProviderWithTopicManagement) TopicExists(ctx context.Context, topic string) (bool, error) {
	m.existsCalled = true
	_, exists := m.topics[topic]
	return exists || m.shouldReturnExists, nil
}

func (m *mockProviderWithTopicManagement) ListTopics(ctx context.Context) ([]string, error) {
	m.listCalled = true
	topics := make([]string, 0, len(m.topics))
	for topic := range m.topics {
		topics = append(topics, topic)
	}
	return topics, nil
}

func (m *mockProviderWithTopicManagement) UpdateTopicConfig(ctx context.Context, topic string, config *provider.TopicConfig) error {
	m.updateCalled = true
	if _, exists := m.topics[topic]; !exists {
		return provider.ErrTopicNotFound
	}
	m.topics[topic] = config
	return nil
}

func TestHeimdall_SupportsTopicManagement(t *testing.T) {
	tests := []struct {
		name     string
		provider provider.Provider
		want     bool
	}{
		{
			name:     "provider_with_topic_management",
			provider: &mockProviderWithTopicManagement{},
			want:     true,
		},
		{
			name:     "provider_without_topic_management",
			provider: &mockProvider{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Heimdall{
				provider: tt.provider,
			}

			got := h.SupportsTopicManagement()
			if got != tt.want {
				t.Errorf("SupportsTopicManagement() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHeimdall_CreateTopic(t *testing.T) {
	tests := []struct {
		name        string
		provider    provider.Provider
		config      *provider.TopicConfig
		expectError bool
		errorIs     error
	}{
		{
			name:     "successful_create",
			provider: &mockProviderWithTopicManagement{},
			config: &provider.TopicConfig{
				Name:       "test-topic",
				Partitions: 3,
			},
			expectError: false,
		},
		{
			name:     "provider_not_support_topic_management",
			provider: &mockProvider{},
			config: &provider.TopicConfig{
				Name: "test-topic",
			},
			expectError: true,
			errorIs:     provider.ErrTopicManagementNotSupported,
		},
		{
			name: "topic_already_exists",
			provider: &mockProviderWithTopicManagement{
				topics: map[string]*provider.TopicConfig{
					"existing-topic": {Name: "existing-topic"},
				},
			},
			config: &provider.TopicConfig{
				Name: "existing-topic",
			},
			expectError: true,
			errorIs:     provider.ErrTopicAlreadyExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Heimdall{
				provider: tt.provider,
			}

			err := h.CreateTopic(context.Background(), tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errorIs != nil && err != tt.errorIs {
					t.Errorf("Expected error %v, got %v", tt.errorIs, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify the mock was called
				if mock, ok := tt.provider.(*mockProviderWithTopicManagement); ok {
					if !mock.createCalled {
						t.Error("CreateTopic was not called on provider")
					}
				}
			}
		})
	}
}

func TestHeimdall_DeleteTopic(t *testing.T) {
	tests := []struct {
		name        string
		provider    provider.Provider
		topic       string
		expectError bool
		errorIs     error
	}{
		{
			name: "successful_delete",
			provider: &mockProviderWithTopicManagement{
				topics: map[string]*provider.TopicConfig{
					"test-topic": {Name: "test-topic"},
				},
			},
			topic:       "test-topic",
			expectError: false,
		},
		{
			name:        "provider_not_support_topic_management",
			provider:    &mockProvider{},
			topic:       "test-topic",
			expectError: true,
			errorIs:     provider.ErrTopicManagementNotSupported,
		},
		{
			name:        "topic_not_found",
			provider:    &mockProviderWithTopicManagement{},
			topic:       "nonexistent",
			expectError: true,
			errorIs:     provider.ErrTopicNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Heimdall{
				provider: tt.provider,
			}

			err := h.DeleteTopic(context.Background(), tt.topic)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errorIs != nil && err != tt.errorIs {
					t.Errorf("Expected error %v, got %v", tt.errorIs, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify the mock was called
				if mock, ok := tt.provider.(*mockProviderWithTopicManagement); ok {
					if !mock.deleteCalled {
						t.Error("DeleteTopic was not called on provider")
					}
				}
			}
		})
	}
}

func TestHeimdall_TopicExists(t *testing.T) {
	tests := []struct {
		name        string
		provider    provider.Provider
		topic       string
		want        bool
		expectError bool
		errorIs     error
	}{
		{
			name: "topic_exists",
			provider: &mockProviderWithTopicManagement{
				topics: map[string]*provider.TopicConfig{
					"test-topic": {Name: "test-topic"},
				},
			},
			topic:       "test-topic",
			want:        true,
			expectError: false,
		},
		{
			name:        "topic_not_exists",
			provider:    &mockProviderWithTopicManagement{},
			topic:       "nonexistent",
			want:        false,
			expectError: false,
		},
		{
			name:        "provider_not_support_topic_management",
			provider:    &mockProvider{},
			topic:       "test-topic",
			want:        false,
			expectError: true,
			errorIs:     provider.ErrTopicManagementNotSupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Heimdall{
				provider: tt.provider,
			}

			exists, err := h.TopicExists(context.Background(), tt.topic)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errorIs != nil && err != tt.errorIs {
					t.Errorf("Expected error %v, got %v", tt.errorIs, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if exists != tt.want {
					t.Errorf("TopicExists() = %v, want %v", exists, tt.want)
				}

				// Verify the mock was called
				if mock, ok := tt.provider.(*mockProviderWithTopicManagement); ok {
					if !mock.existsCalled {
						t.Error("TopicExists was not called on provider")
					}
				}
			}
		})
	}
}

func TestHeimdall_ListTopics(t *testing.T) {
	tests := []struct {
		name        string
		provider    provider.Provider
		want        []string
		expectError bool
		errorIs     error
	}{
		{
			name: "successful_list",
			provider: &mockProviderWithTopicManagement{
				topics: map[string]*provider.TopicConfig{
					"topic1": {Name: "topic1"},
					"topic2": {Name: "topic2"},
				},
			},
			want:        []string{"topic1", "topic2"},
			expectError: false,
		},
		{
			name:        "empty_list",
			provider:    &mockProviderWithTopicManagement{},
			want:        []string{},
			expectError: false,
		},
		{
			name:        "provider_not_support_topic_management",
			provider:    &mockProvider{},
			want:        nil,
			expectError: true,
			errorIs:     provider.ErrTopicManagementNotSupported,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Heimdall{
				provider: tt.provider,
			}

			topics, err := h.ListTopics(context.Background())

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errorIs != nil && err != tt.errorIs {
					t.Errorf("Expected error %v, got %v", tt.errorIs, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				if len(topics) != len(tt.want) {
					t.Errorf("ListTopics() returned %d topics, want %d", len(topics), len(tt.want))
				}

				// Verify the mock was called
				if mock, ok := tt.provider.(*mockProviderWithTopicManagement); ok {
					if !mock.listCalled {
						t.Error("ListTopics was not called on provider")
					}
				}
			}
		})
	}
}

func TestHeimdall_UpdateTopicConfig(t *testing.T) {
	tests := []struct {
		name        string
		provider    provider.Provider
		topic       string
		config      *provider.TopicConfig
		expectError bool
		errorIs     error
	}{
		{
			name: "successful_update",
			provider: &mockProviderWithTopicManagement{
				topics: map[string]*provider.TopicConfig{
					"test-topic": {Name: "test-topic", Partitions: 1},
				},
			},
			topic: "test-topic",
			config: &provider.TopicConfig{
				Name:       "test-topic",
				Partitions: 3,
			},
			expectError: false,
		},
		{
			name:     "provider_not_support_topic_management",
			provider: &mockProvider{},
			topic:    "test-topic",
			config: &provider.TopicConfig{
				Name: "test-topic",
			},
			expectError: true,
			errorIs:     provider.ErrTopicManagementNotSupported,
		},
		{
			name:     "topic_not_found",
			provider: &mockProviderWithTopicManagement{},
			topic:    "nonexistent",
			config: &provider.TopicConfig{
				Name: "nonexistent",
			},
			expectError: true,
			errorIs:     provider.ErrTopicNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := &Heimdall{
				provider: tt.provider,
			}

			err := h.UpdateTopicConfig(context.Background(), tt.topic, tt.config)

			if tt.expectError {
				if err == nil {
					t.Error("Expected error, got nil")
				} else if tt.errorIs != nil && err != tt.errorIs {
					t.Errorf("Expected error %v, got %v", tt.errorIs, err)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error: %v", err)
				}

				// Verify the mock was called
				if mock, ok := tt.provider.(*mockProviderWithTopicManagement); ok {
					if !mock.updateCalled {
						t.Error("UpdateTopicConfig was not called on provider")
					}
				}
			}
		})
	}
}

func TestHeimdall_TopicManagement_ClosedInstance(t *testing.T) {
	h := &Heimdall{
		provider: &mockProviderWithTopicManagement{},
		closed:   true,
	}

	ctx := context.Background()
	config := &provider.TopicConfig{Name: "test"}

	// Test all topic management methods with closed instance
	if err := h.CreateTopic(ctx, config); err == nil {
		t.Error("CreateTopic should fail with closed instance")
	}

	if err := h.DeleteTopic(ctx, "test"); err == nil {
		t.Error("DeleteTopic should fail with closed instance")
	}

	if _, err := h.TopicExists(ctx, "test"); err == nil {
		t.Error("TopicExists should fail with closed instance")
	}

	if _, err := h.ListTopics(ctx); err == nil {
		t.Error("ListTopics should fail with closed instance")
	}

	if err := h.UpdateTopicConfig(ctx, "test", config); err == nil {
		t.Error("UpdateTopicConfig should fail with closed instance")
	}
}
