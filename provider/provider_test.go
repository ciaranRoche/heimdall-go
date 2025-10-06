package provider

import (
	"context"
	"fmt"
	"testing"
)

// testProvider is a simple test implementation
type testProvider struct {
	name string
}

func (t *testProvider) Publish(ctx context.Context, topic string, data []byte, headers map[string]interface{}, correlationID string) error {
	return nil
}

func (t *testProvider) Subscribe(ctx context.Context, topic string, handler MessageHandler) error {
	return nil
}

func (t *testProvider) HealthCheck(ctx context.Context) error {
	return nil
}

func (t *testProvider) Close() error {
	return nil
}

func TestRegister(t *testing.T) {
	// Clear registry for test
	mu.Lock()
	registry = make(map[string]Factory)
	mu.Unlock()

	factory := func(config map[string]interface{}) (Provider, error) {
		return &testProvider{name: "test"}, nil
	}

	Register("test-provider", factory)

	if !IsRegistered("test-provider") {
		t.Error("Expected provider to be registered")
	}

	if IsRegistered("nonexistent") {
		t.Error("Expected nonexistent provider to not be registered")
	}
}

func TestNewProvider(t *testing.T) {
	// Clear and setup registry for test
	mu.Lock()
	registry = make(map[string]Factory)
	mu.Unlock()

	factory := func(config map[string]interface{}) (Provider, error) {
		return &testProvider{name: "test"}, nil
	}

	Register("test-new", factory)

	tests := []struct {
		name       string
		provider   string
		config     map[string]interface{}
		wantErr    bool
		errMessage string
	}{
		{
			name:     "valid provider",
			provider: "test-new",
			config:   map[string]interface{}{},
			wantErr:  false,
		},
		{
			name:       "unregistered provider",
			provider:   "nonexistent",
			config:     map[string]interface{}{},
			wantErr:    true,
			errMessage: "not found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, err := NewProvider(tt.provider, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewProvider() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !tt.wantErr && prov == nil {
				t.Error("Expected provider to be returned")
			}

			if tt.wantErr && tt.errMessage != "" && err != nil {
				if err.Error() == "" {
					t.Errorf("Expected error message containing '%s', got empty error", tt.errMessage)
				}
			}
		})
	}
}

func TestListProviders(t *testing.T) {
	// Clear and setup registry for test
	mu.Lock()
	registry = make(map[string]Factory)
	mu.Unlock()

	// Register multiple providers
	factory1 := func(config map[string]interface{}) (Provider, error) {
		return &testProvider{name: "provider1"}, nil
	}
	factory2 := func(config map[string]interface{}) (Provider, error) {
		return &testProvider{name: "provider2"}, nil
	}

	Register("provider1", factory1)
	Register("provider2", factory2)

	providers := ListProviders()

	if len(providers) != 2 {
		t.Errorf("Expected 2 providers, got %d", len(providers))
	}

	// Check that both providers are in the list
	found := make(map[string]bool)
	for _, p := range providers {
		found[p] = true
	}

	if !found["provider1"] {
		t.Error("Expected provider1 in list")
	}
	if !found["provider2"] {
		t.Error("Expected provider2 in list")
	}
}

func TestIsRegistered(t *testing.T) {
	// Clear and setup registry for test
	mu.Lock()
	registry = make(map[string]Factory)
	mu.Unlock()

	factory := func(config map[string]interface{}) (Provider, error) {
		return &testProvider{name: "test"}, nil
	}

	if IsRegistered("test-registered") {
		t.Error("Expected provider to not be registered yet")
	}

	Register("test-registered", factory)

	if !IsRegistered("test-registered") {
		t.Error("Expected provider to be registered")
	}
}

func TestProviderReregistration(t *testing.T) {
	// Clear registry for test
	mu.Lock()
	registry = make(map[string]Factory)
	mu.Unlock()

	factory1 := func(config map[string]interface{}) (Provider, error) {
		return &testProvider{name: "first"}, nil
	}
	factory2 := func(config map[string]interface{}) (Provider, error) {
		return &testProvider{name: "second"}, nil
	}

	// Register first version
	Register("reregister", factory1)

	prov1, err := NewProvider("reregister", nil)
	if err != nil {
		t.Fatalf("Failed to create first provider: %v", err)
	}
	if tp, ok := prov1.(*testProvider); !ok || tp.name != "first" {
		t.Error("Expected first provider")
	}

	// Re-register with new factory (should overwrite)
	Register("reregister", factory2)

	prov2, err := NewProvider("reregister", nil)
	if err != nil {
		t.Fatalf("Failed to create second provider: %v", err)
	}
	if tp, ok := prov2.(*testProvider); !ok || tp.name != "second" {
		t.Error("Expected second provider after re-registration")
	}
}

func TestFactoryError(t *testing.T) {
	// Clear registry for test
	mu.Lock()
	registry = make(map[string]Factory)
	mu.Unlock()

	// Register a factory that returns an error
	factory := func(config map[string]interface{}) (Provider, error) {
		return nil, fmt.Errorf("factory error")
	}

	Register("error-provider", factory)

	_, err := NewProvider("error-provider", nil)
	if err == nil {
		t.Error("Expected error from factory")
	}
}

func TestConcurrentRegister(t *testing.T) {
	// Clear registry for test
	mu.Lock()
	registry = make(map[string]Factory)
	mu.Unlock()

	factory := func(config map[string]interface{}) (Provider, error) {
		return &testProvider{}, nil
	}

	// Test concurrent registrations
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(i int) {
			Register(fmt.Sprintf("concurrent-%d", i), factory)
			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all providers were registered
	providers := ListProviders()
	if len(providers) != 10 {
		t.Errorf("Expected 10 providers, got %d", len(providers))
	}
}

func TestConcurrentNewProvider(t *testing.T) {
	// Clear and setup registry for test
	mu.Lock()
	registry = make(map[string]Factory)
	mu.Unlock()

	factory := func(config map[string]interface{}) (Provider, error) {
		return &testProvider{}, nil
	}

	Register("concurrent-new", factory)

	// Test concurrent provider creation
	done := make(chan bool)
	errors := make(chan error, 10)

	for i := 0; i < 10; i++ {
		go func() {
			_, err := NewProvider("concurrent-new", nil)
			if err != nil {
				errors <- err
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Unexpected error in concurrent NewProvider: %v", err)
	}
}
