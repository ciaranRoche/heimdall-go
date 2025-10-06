package heimdall

import (
	"context"
	"testing"
	"time"

	"github.com/ciaranRoche/heimdall-go/provider"
)

// mockProvider is a test implementation of provider.Provider
type mockProvider struct {
	publishCalled   bool
	subscribeCalled bool
	closeCalled     bool
	publishErr      error
	subscribeErr    error
}

func (m *mockProvider) Publish(ctx context.Context, topic string, data []byte, headers map[string]interface{}, correlationID string) error {
	m.publishCalled = true
	return m.publishErr
}

func (m *mockProvider) Subscribe(ctx context.Context, topic string, handler provider.MessageHandler) error {
	m.subscribeCalled = true
	return m.subscribeErr
}

func (m *mockProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockProvider) Close() error {
	m.closeCalled = true
	return nil
}

func TestNew(t *testing.T) {
	// Register mock provider
	provider.Register("mock", func(config map[string]interface{}) (provider.Provider, error) {
		return &mockProvider{}, nil
	})

	tests := []struct {
		name    string
		opts    []Option
		wantErr bool
	}{
		{
			name: "valid configuration",
			opts: []Option{
				WithProvider("mock"),
				WithConfig(map[string]interface{}{}),
			},
			wantErr: false,
		},
		{
			name: "with consumer group",
			opts: []Option{
				WithProvider("mock"),
				WithConsumerGroup("test-group"),
			},
			wantErr: false,
		},
		{
			name: "unregistered provider",
			opts: []Option{
				WithProvider("nonexistent"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h, err := New(tt.opts...)
			if (err != nil) != tt.wantErr {
				t.Errorf("New() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && h != nil {
				h.Close()
			}
		})
	}
}

func TestHeimdall_Publish(t *testing.T) {
	mock := &mockProvider{}
	provider.Register("test-publish", func(config map[string]interface{}) (provider.Provider, error) {
		return mock, nil
	})

	h, err := New(WithProvider("test-publish"))
	if err != nil {
		t.Fatalf("Failed to create Heimdall: %v", err)
	}
	defer h.Close()

	ctx := context.Background()
	err = h.Publish(ctx, "test.topic", []byte("test message"))
	if err != nil {
		t.Errorf("Publish() error = %v", err)
	}

	if !mock.publishCalled {
		t.Error("Expected provider.Publish to be called")
	}
}

func TestHeimdall_PublishWithOptions(t *testing.T) {
	mock := &mockProvider{}
	provider.Register("test-publish-opts", func(config map[string]interface{}) (provider.Provider, error) {
		return mock, nil
	})

	h, err := New(WithProvider("test-publish-opts"))
	if err != nil {
		t.Fatalf("Failed to create Heimdall: %v", err)
	}
	defer h.Close()

	ctx := context.Background()
	err = h.Publish(ctx, "test.topic", []byte("test message"),
		WithCorrelationID("test-123"),
		WithHeaders(map[string]interface{}{
			"content-type": "application/json",
		}),
	)

	if err != nil {
		t.Errorf("Publish() error = %v", err)
	}

	if !mock.publishCalled {
		t.Error("Expected provider.Publish to be called")
	}
}

func TestHeimdall_Subscribe(t *testing.T) {
	mock := &mockProvider{}
	provider.Register("test-subscribe", func(config map[string]interface{}) (provider.Provider, error) {
		return mock, nil
	})

	h, err := New(WithProvider("test-subscribe"))
	if err != nil {
		t.Fatalf("Failed to create Heimdall: %v", err)
	}
	defer h.Close()

	ctx := context.Background()

	cancel, err := h.Subscribe(ctx, "test.topic", func(ctx context.Context, msg *Message) error {
		// Handler will be called when messages arrive
		return nil
	})

	if err != nil {
		t.Errorf("Subscribe() error = %v", err)
	}

	if !mock.subscribeCalled {
		t.Error("Expected provider.Subscribe to be called")
	}

	if cancel == nil {
		t.Error("Expected cancel function to be returned")
	}

	// Cancel subscription
	if cancel != nil {
		cancel()
	}
}

func TestHeimdall_HealthCheck(t *testing.T) {
	mock := &mockProvider{}
	provider.Register("test-health", func(config map[string]interface{}) (provider.Provider, error) {
		return mock, nil
	})

	h, err := New(WithProvider("test-health"))
	if err != nil {
		t.Fatalf("Failed to create Heimdall: %v", err)
	}
	defer h.Close()

	ctx := context.Background()
	err = h.HealthCheck(ctx)
	if err != nil {
		t.Errorf("HealthCheck() error = %v", err)
	}
}

func TestHeimdall_Close(t *testing.T) {
	mock := &mockProvider{}
	provider.Register("test-close", func(config map[string]interface{}) (provider.Provider, error) {
		return mock, nil
	})

	h, err := New(WithProvider("test-close"))
	if err != nil {
		t.Fatalf("Failed to create Heimdall: %v", err)
	}

	err = h.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}

	if !mock.closeCalled {
		t.Error("Expected provider.Close to be called")
	}

	// Test operations after close
	ctx := context.Background()
	err = h.Publish(ctx, "test", []byte("data"))
	if err == nil {
		t.Error("Expected error when publishing to closed instance")
	}

	_, err = h.Subscribe(ctx, "test", func(ctx context.Context, msg *Message) error {
		return nil
	})
	if err == nil {
		t.Error("Expected error when subscribing to closed instance")
	}
}

func TestGetCorrelationID(t *testing.T) {
	tests := []struct {
		name    string
		headers map[string]interface{}
		want    string
	}{
		{
			name: "correlation_id present",
			headers: map[string]interface{}{
				"correlation_id": "test-123",
			},
			want: "test-123",
		},
		{
			name: "correlationId present",
			headers: map[string]interface{}{
				"correlationId": "test-456",
			},
			want: "test-456",
		},
		{
			name: "x-correlation-id present",
			headers: map[string]interface{}{
				"x-correlation-id": "test-789",
			},
			want: "test-789",
		},
		{
			name:    "no correlation id",
			headers: map[string]interface{}{},
			want:    "",
		},
		{
			name:    "nil headers",
			headers: nil,
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getCorrelationID(tt.headers)
			if got != tt.want {
				t.Errorf("getCorrelationID() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				Provider: "kafka",
				Config:   map[string]interface{}{},
			},
			wantErr: false,
		},
		{
			name: "missing provider",
			config: &Config{
				Provider: "",
			},
			wantErr: true,
		},
		{
			name: "nil config map",
			config: &Config{
				Provider: "kafka",
				Config:   nil,
			},
			wantErr: false, // Should initialize empty map
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestMessage(t *testing.T) {
	msg := &Message{
		Topic:         "test.topic",
		Data:          []byte("test data"),
		CorrelationID: "test-123",
		Headers: map[string]interface{}{
			"key": "value",
		},
		Metadata: map[string]interface{}{
			"meta": "data",
		},
	}

	if msg.Topic != "test.topic" {
		t.Errorf("Expected topic 'test.topic', got '%s'", msg.Topic)
	}

	if string(msg.Data) != "test data" {
		t.Errorf("Expected data 'test data', got '%s'", string(msg.Data))
	}

	if msg.CorrelationID != "test-123" {
		t.Errorf("Expected correlation ID 'test-123', got '%s'", msg.CorrelationID)
	}
}

func TestConcurrentOperations(t *testing.T) {
	mock := &mockProvider{}
	provider.Register("test-concurrent", func(config map[string]interface{}) (provider.Provider, error) {
		return mock, nil
	})

	h, err := New(WithProvider("test-concurrent"))
	if err != nil {
		t.Fatalf("Failed to create Heimdall: %v", err)
	}
	defer h.Close()

	ctx := context.Background()

	// Test concurrent publishes
	for i := 0; i < 10; i++ {
		go func(i int) {
			_ = h.Publish(ctx, "test.topic", []byte("message"))
		}(i)
	}

	// Give goroutines time to execute
	time.Sleep(100 * time.Millisecond)
}
