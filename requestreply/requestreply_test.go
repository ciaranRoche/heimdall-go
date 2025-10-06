package requestreply

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ciaranRoche/heimdall-go"
)

// mockHeimdall implements a mock Heimdall client for testing
type mockHeimdall struct {
	mu               sync.Mutex
	published        []heimdall.Message
	subscriptions    map[string]heimdall.MessageHandler
	publishErr       error
	subscribeErr     error
	autoRespond      bool
	responseDelay    time.Duration
	responseData     []byte
	responseHeaders  map[string]any
	responseCorrelID string
}

func newMockHeimdall() *mockHeimdall {
	return &mockHeimdall{
		subscriptions: make(map[string]heimdall.MessageHandler),
	}
}

func (m *mockHeimdall) Publish(ctx context.Context, topic string, data []byte, opts ...heimdall.PublishOption) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.publishErr != nil {
		return m.publishErr
	}

	pubOpts := &heimdall.PublishOptions{}
	for _, opt := range opts {
		opt(pubOpts)
	}

	msg := heimdall.Message{
		Topic:         topic,
		Data:          data,
		Headers:       pubOpts.Headers,
		CorrelationID: pubOpts.CorrelationID,
	}

	m.published = append(m.published, msg)

	// Auto-respond if configured
	if m.autoRespond {
		go func() {
			if m.responseDelay > 0 {
				time.Sleep(m.responseDelay)
			}

			responseTopic, _ := msg.Headers["response_topic"].(string)
			correlationID := msg.CorrelationID

			if m.responseCorrelID != "" {
				correlationID = m.responseCorrelID
			}

			m.mu.Lock()
			handler, exists := m.subscriptions[responseTopic]
			headers := m.responseHeaders
			if headers == nil {
				headers = make(map[string]any)
			}
			data := m.responseData
			m.mu.Unlock()

			if exists && handler != nil {
				_ = handler(context.Background(), &heimdall.Message{
					Topic:         responseTopic,
					Data:          data,
					Headers:       headers,
					CorrelationID: correlationID,
				})
			}
		}()
	}

	return nil
}

func (m *mockHeimdall) Subscribe(ctx context.Context, topic string, handler heimdall.MessageHandler) (context.CancelFunc, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.subscribeErr != nil {
		return nil, m.subscribeErr
	}

	m.subscriptions[topic] = handler
	return func() {}, nil
}

func (m *mockHeimdall) Close() error {
	return nil
}

func (m *mockHeimdall) HealthCheck(ctx context.Context) error {
	return nil
}

func (m *mockHeimdall) getPublishedCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.published)
}

func (m *mockHeimdall) getLastPublished() *heimdall.Message {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.published) == 0 {
		return nil
	}
	return &m.published[len(m.published)-1]
}

func TestNewClient(t *testing.T) {
	mock := newMockHeimdall()
	client := NewClient(mock)

	if client == nil {
		t.Fatal("Expected client to be created")
	}

	if client.PendingRequests() != 0 {
		t.Errorf("Expected 0 pending requests, got %d", client.PendingRequests())
	}
}

func TestClient_Request_Success(t *testing.T) {
	mock := newMockHeimdall()
	mock.autoRespond = true
	mock.responseData = []byte("response data")

	client := NewClient(mock)

	req := &Request{
		Topic:         "request.topic",
		Data:          []byte("request data"),
		ResponseTopic: "response.topic",
		Timeout:       5 * time.Second,
	}

	response, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if response == nil {
		t.Fatal("Expected response, got nil")
	}

	if string(response.Data) != "response data" {
		t.Errorf("Expected 'response data', got '%s'", string(response.Data))
	}

	if response.CorrelationID == "" {
		t.Error("Expected correlation ID in response")
	}

	// Verify request was published
	if mock.getPublishedCount() != 1 {
		t.Errorf("Expected 1 published message, got %d", mock.getPublishedCount())
	}

	published := mock.getLastPublished()
	if published.Topic != "request.topic" {
		t.Errorf("Expected topic 'request.topic', got '%s'", published.Topic)
	}

	if published.CorrelationID == "" {
		t.Error("Expected correlation ID in published message")
	}
}

func TestClient_Request_Timeout(t *testing.T) {
	mock := newMockHeimdall()
	// Don't auto-respond, will timeout

	client := NewClient(mock)

	req := &Request{
		Topic:         "request.topic",
		Data:          []byte("request data"),
		ResponseTopic: "response.topic",
		Timeout:       100 * time.Millisecond,
	}

	start := time.Now()
	response, err := client.Request(context.Background(), req)
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("Expected timeout error, got nil")
	}

	if response != nil {
		t.Errorf("Expected nil response on timeout, got %v", response)
	}

	// Should timeout around 100ms
	if elapsed < 100*time.Millisecond || elapsed > 200*time.Millisecond {
		t.Errorf("Expected timeout around 100ms, took %v", elapsed)
	}

	// Verify pending request was cleaned up
	if client.PendingRequests() != 0 {
		t.Errorf("Expected 0 pending requests after timeout, got %d", client.PendingRequests())
	}
}

func TestClient_Request_ContextCancelled(t *testing.T) {
	mock := newMockHeimdall()
	mock.autoRespond = true
	mock.responseDelay = 200 * time.Millisecond

	client := NewClient(mock)

	ctx, cancel := context.WithCancel(context.Background())

	req := &Request{
		Topic:         "request.topic",
		Data:          []byte("request data"),
		ResponseTopic: "response.topic",
		Timeout:       5 * time.Second,
	}

	// Cancel context after 50ms
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	response, err := client.Request(ctx, req)
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}

	if response != nil {
		t.Errorf("Expected nil response on cancel, got %v", response)
	}
}

func TestClient_Request_CustomCorrelationID(t *testing.T) {
	mock := newMockHeimdall()
	mock.autoRespond = true
	mock.responseData = []byte("response")

	client := NewClient(mock)

	req := &Request{
		Topic:         "request.topic",
		Data:          []byte("request data"),
		ResponseTopic: "response.topic",
		CorrelationID: "custom-correlation-123",
		Timeout:       5 * time.Second,
	}

	response, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if response.CorrelationID != "custom-correlation-123" {
		t.Errorf("Expected correlation ID 'custom-correlation-123', got '%s'", response.CorrelationID)
	}

	published := mock.getLastPublished()
	if published.CorrelationID != "custom-correlation-123" {
		t.Errorf("Expected published correlation ID 'custom-correlation-123', got '%s'", published.CorrelationID)
	}
}

func TestClient_Request_WithHeaders(t *testing.T) {
	mock := newMockHeimdall()
	mock.autoRespond = true

	client := NewClient(mock)

	req := &Request{
		Topic:         "request.topic",
		Data:          []byte("request data"),
		ResponseTopic: "response.topic",
		Headers: map[string]any{
			"x-custom": "value",
			"x-user":   "test-user",
		},
		Timeout: 5 * time.Second,
	}

	_, err := client.Request(context.Background(), req)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	published := mock.getLastPublished()
	if published.Headers["x-custom"] != "value" {
		t.Errorf("Expected header x-custom=value, got %v", published.Headers["x-custom"])
	}

	// Verify correlation_id and response_topic were added
	if published.Headers["correlation_id"] == "" {
		t.Error("Expected correlation_id header")
	}

	if published.Headers["response_topic"] != "response.topic" {
		t.Errorf("Expected response_topic header 'response.topic', got %v", published.Headers["response_topic"])
	}
}

func TestClient_RequestAsync_Success(t *testing.T) {
	mock := newMockHeimdall()
	mock.autoRespond = true
	mock.responseData = []byte("async response")

	client := NewClient(mock)

	var wg sync.WaitGroup
	wg.Add(1)

	var receivedResponse *heimdall.Message
	handler := func(ctx context.Context, msg *heimdall.Message) error {
		receivedResponse = msg
		wg.Done()
		return nil
	}

	req := &Request{
		Topic:         "request.topic",
		Data:          []byte("request data"),
		ResponseTopic: "response.topic",
		Timeout:       5 * time.Second,
	}

	err := client.RequestAsync(context.Background(), req, handler)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Wait for response
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		// Success
	case <-time.After(1 * time.Second):
		t.Fatal("Timeout waiting for async response")
	}

	if string(receivedResponse.Data) != "async response" {
		t.Errorf("Expected 'async response', got '%s'", string(receivedResponse.Data))
	}
}

func TestClient_Request_ErrorResponse(t *testing.T) {
	mock := newMockHeimdall()
	mock.autoRespond = true
	mock.responseHeaders = map[string]any{
		"error": "something went wrong",
	}

	client := NewClient(mock)

	req := &Request{
		Topic:         "request.topic",
		Data:          []byte("request data"),
		ResponseTopic: "response.topic",
		Timeout:       5 * time.Second,
	}

	response, err := client.Request(context.Background(), req)
	if err == nil {
		t.Error("Expected error in response")
	}

	if response == nil {
		t.Fatal("Expected response even with error")
	}

	if response.Error == nil {
		t.Error("Expected response.Error to be set")
	}
}

func TestClient_ConcurrentRequests(t *testing.T) {
	mock := newMockHeimdall()
	mock.autoRespond = true
	mock.responseData = []byte("response")

	client := NewClient(mock)

	concurrency := 10
	var wg sync.WaitGroup
	wg.Add(concurrency)

	errors := make(chan error, concurrency)

	for i := 0; i < concurrency; i++ {
		go func(_ int) {
			defer wg.Done()

			req := &Request{
				Topic:         "request.topic",
				Data:          []byte("request"),
				ResponseTopic: "response.topic",
				Timeout:       5 * time.Second,
			}

			_, err := client.Request(context.Background(), req)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent request failed: %v", err)
	}

	// All requests should be cleaned up
	if client.PendingRequests() != 0 {
		t.Errorf("Expected 0 pending requests, got %d", client.PendingRequests())
	}
}

func TestClient_Stats(t *testing.T) {
	mock := newMockHeimdall()
	client := NewClient(mock)

	stats := client.Stats()

	if stats["pending_requests"] != 0 {
		t.Errorf("Expected 0 pending_requests, got %v", stats["pending_requests"])
	}

	if len(stats["subscribed_topics"].([]string)) != 0 {
		t.Errorf("Expected 0 subscribed_topics, got %v", stats["subscribed_topics"])
	}
}

func TestClient_Close(t *testing.T) {
	mock := newMockHeimdall()
	mock.autoRespond = true
	mock.responseDelay = 1 * time.Second // Long delay

	client := NewClient(mock)

	// Start a request that won't complete
	go func() {
		req := &Request{
			Topic:         "request.topic",
			Data:          []byte("request"),
			ResponseTopic: "response.topic",
			Timeout:       5 * time.Second,
		}
		_, _ = client.Request(context.Background(), req)
	}()

	// Wait for request to start
	time.Sleep(50 * time.Millisecond)

	// Verify there's a pending request
	if client.PendingRequests() == 0 {
		t.Error("Expected pending request before close")
	}

	// Close client
	err := client.Close()
	if err != nil {
		t.Errorf("Expected no error on close, got %v", err)
	}

	// Verify cleanup
	if client.PendingRequests() != 0 {
		t.Errorf("Expected 0 pending requests after close, got %d", client.PendingRequests())
	}
}
