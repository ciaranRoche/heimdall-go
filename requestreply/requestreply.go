// Package requestreply provides request/reply pattern support for Heimdall messaging.
//
// The request/reply pattern allows sending a request message and waiting for a response,
// with automatic correlation ID tracking and response routing.
package requestreply

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ciaranRoche/heimdall-go"
	"github.com/google/uuid"
)

// ResponseHandler is called when a response is received for a request.
type ResponseHandler func(ctx context.Context, response *heimdall.Message) error

// Request represents a request message with correlation tracking.
type Request struct {
	// Topic to publish the request to
	Topic string

	// Data is the request payload
	Data []byte

	// Headers are optional message headers
	Headers map[string]any

	// Metadata is optional message metadata
	Metadata map[string]any

	// ResponseTopic is the topic to listen for the response on
	ResponseTopic string

	// Timeout is how long to wait for a response (default: 30s)
	Timeout time.Duration

	// CorrelationID is the unique identifier for this request (auto-generated if empty)
	CorrelationID string
}

// Response represents a response message with correlation tracking.
type Response struct {
	// CorrelationID links this response to the original request
	CorrelationID string

	// Data is the response payload
	Data []byte

	// Headers are message headers
	Headers map[string]any

	// Metadata is message metadata
	Metadata map[string]any

	// Error indicates if the response represents an error
	Error error
}

// pendingRequest tracks an in-flight request awaiting a response.
type pendingRequest struct {
	correlationID string
	responseChan  chan Response
	timeout       time.Duration
	cancelFunc    context.CancelFunc
}

// Publisher defines the interface for publishing messages.
type Publisher interface {
	Publish(ctx context.Context, topic string, data []byte, opts ...heimdall.PublishOption) error
}

// Subscriber defines the interface for subscribing to messages.
type Subscriber interface {
	Subscribe(ctx context.Context, topic string, handler heimdall.MessageHandler) (context.CancelFunc, error)
}

// MessagingClient combines Publisher and Subscriber interfaces.
type MessagingClient interface {
	Publisher
	Subscriber
}

// Client provides request/reply pattern functionality.
type Client struct {
	client          MessagingClient
	pendingRequests map[string]*pendingRequest
	mu              sync.RWMutex
	subscribers     map[string]context.CancelFunc // response topic -> cancel func
	subMu           sync.RWMutex
}

// NewClient creates a new request/reply client.
func NewClient(client MessagingClient) *Client {
	return &Client{
		client:          client,
		pendingRequests: make(map[string]*pendingRequest),
		subscribers:     make(map[string]context.CancelFunc),
	}
}

// Request sends a request and waits for a response.
func (c *Client) Request(ctx context.Context, req *Request) (*Response, error) {
	// Generate correlation ID if not provided
	if req.CorrelationID == "" {
		req.CorrelationID = uuid.New().String()
	}

	// Set default timeout
	if req.Timeout == 0 {
		req.Timeout = 30 * time.Second
	}

	// Ensure response topic subscription
	if err := c.ensureResponseSubscription(req.ResponseTopic); err != nil {
		return nil, fmt.Errorf("failed to subscribe to response topic: %w", err)
	}

	// Create response channel for this request
	responseChan := make(chan Response, 1)
	reqCtx, cancel := context.WithTimeout(ctx, req.Timeout)

	pending := &pendingRequest{
		correlationID: req.CorrelationID,
		responseChan:  responseChan,
		timeout:       req.Timeout,
		cancelFunc:    cancel,
	}

	// Register pending request
	c.mu.Lock()
	c.pendingRequests[req.CorrelationID] = pending
	c.mu.Unlock()

	// Ensure cleanup on exit
	defer func() {
		c.mu.Lock()
		delete(c.pendingRequests, req.CorrelationID)
		c.mu.Unlock()
		cancel()
	}()

	// Publish request with correlation ID
	headers := make(map[string]any)
	for k, v := range req.Headers {
		headers[k] = v
	}
	headers["correlation_id"] = req.CorrelationID
	headers["response_topic"] = req.ResponseTopic

	err := c.client.Publish(ctx, req.Topic, req.Data,
		heimdall.WithHeaders(headers),
		heimdall.WithCorrelationID(req.CorrelationID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to publish request: %w", err)
	}

	// Wait for response or timeout
	select {
	case response := <-responseChan:
		return &response, response.Error
	case <-reqCtx.Done():
		return nil, fmt.Errorf("request timed out after %v", req.Timeout)
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// RequestAsync sends a request and calls the handler when the response arrives.
func (c *Client) RequestAsync(ctx context.Context, req *Request, handler ResponseHandler) error {
	// Generate correlation ID if not provided
	if req.CorrelationID == "" {
		req.CorrelationID = uuid.New().String()
	}

	// Set default timeout
	if req.Timeout == 0 {
		req.Timeout = 30 * time.Second
	}

	// Ensure response topic subscription
	if err := c.ensureResponseSubscription(req.ResponseTopic); err != nil {
		return fmt.Errorf("failed to subscribe to response topic: %w", err)
	}

	// Create response channel for this request
	responseChan := make(chan Response, 1)
	reqCtx, cancel := context.WithTimeout(ctx, req.Timeout)

	pending := &pendingRequest{
		correlationID: req.CorrelationID,
		responseChan:  responseChan,
		timeout:       req.Timeout,
		cancelFunc:    cancel,
	}

	// Register pending request
	c.mu.Lock()
	c.pendingRequests[req.CorrelationID] = pending
	c.mu.Unlock()

	// Start goroutine to handle response
	go func() {
		defer func() {
			c.mu.Lock()
			delete(c.pendingRequests, req.CorrelationID)
			c.mu.Unlock()
			cancel()
		}()

		select {
		case response := <-responseChan:
			msg := &heimdall.Message{
				Topic:         req.ResponseTopic,
				Data:          response.Data,
				Headers:       response.Headers,
				Metadata:      response.Metadata,
				CorrelationID: response.CorrelationID,
			}
			_ = handler(ctx, msg) // Handler error is logged but not returned
		case <-reqCtx.Done():
			// Timeout - call handler with timeout error
			msg := &heimdall.Message{
				Topic:         req.ResponseTopic,
				CorrelationID: req.CorrelationID,
			}
			_ = handler(ctx, msg)
		case <-ctx.Done():
			// Context canceled
			return
		}
	}()

	// Publish request with correlation ID
	headers := make(map[string]any)
	for k, v := range req.Headers {
		headers[k] = v
	}
	headers["correlation_id"] = req.CorrelationID
	headers["response_topic"] = req.ResponseTopic

	err := c.client.Publish(ctx, req.Topic, req.Data,
		heimdall.WithHeaders(headers),
		heimdall.WithCorrelationID(req.CorrelationID),
	)
	if err != nil {
		c.mu.Lock()
		delete(c.pendingRequests, req.CorrelationID)
		c.mu.Unlock()
		cancel()
		return fmt.Errorf("failed to publish request: %w", err)
	}

	return nil
}

// ensureResponseSubscription subscribes to the response topic if not already subscribed.
func (c *Client) ensureResponseSubscription(topic string) error {
	c.subMu.RLock()
	_, exists := c.subscribers[topic]
	c.subMu.RUnlock()

	if exists {
		return nil
	}

	c.subMu.Lock()
	defer c.subMu.Unlock()

	// Double-check after acquiring write lock
	if _, exists := c.subscribers[topic]; exists {
		return nil
	}

	ctx := context.Background()

	cancel, err := c.client.Subscribe(ctx, topic, c.handleResponse)
	if err != nil {
		return err
	}

	c.subscribers[topic] = cancel
	return nil
}

// handleResponse processes incoming response messages.
func (c *Client) handleResponse(ctx context.Context, msg *heimdall.Message) error {
	// Extract correlation ID
	correlationID := msg.CorrelationID
	if correlationID == "" {
		// Try to get from headers
		if cid, ok := msg.Headers["correlation_id"].(string); ok {
			correlationID = cid
		}
	}

	if correlationID == "" {
		// No correlation ID, can't route this response
		return nil
	}

	// Find pending request
	c.mu.RLock()
	pending, exists := c.pendingRequests[correlationID]
	c.mu.RUnlock()

	if !exists {
		// No pending request for this correlation ID (possibly timed out)
		return nil
	}

	// Send response to waiting request
	response := Response{
		CorrelationID: correlationID,
		Data:          msg.Data,
		Headers:       msg.Headers,
		Metadata:      msg.Metadata,
	}

	// Check for error in response
	if errMsg, ok := msg.Headers["error"].(string); ok && errMsg != "" {
		response.Error = fmt.Errorf("%s", errMsg)
	}

	select {
	case pending.responseChan <- response:
		// Response delivered
	default:
		// Channel full or closed, request may have already completed
	}

	return nil
}

// Close stops all response subscriptions and cleans up pending requests.
func (c *Client) Close() error {
	// Cancel all pending requests
	c.mu.Lock()
	for _, pending := range c.pendingRequests {
		pending.cancelFunc()
	}
	c.pendingRequests = make(map[string]*pendingRequest)
	c.mu.Unlock()

	// Cancel all subscriptions
	c.subMu.Lock()
	for _, cancel := range c.subscribers {
		cancel()
	}
	c.subscribers = make(map[string]context.CancelFunc)
	c.subMu.Unlock()

	return nil
}

// PendingRequests returns the number of pending requests.
func (c *Client) PendingRequests() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.pendingRequests)
}

// Stats returns request/reply statistics.
func (c *Client) Stats() map[string]any {
	c.mu.RLock()
	pendingCount := len(c.pendingRequests)
	c.mu.RUnlock()

	c.subMu.RLock()
	subscribedTopics := make([]string, 0, len(c.subscribers))
	for topic := range c.subscribers {
		subscribedTopics = append(subscribedTopics, topic)
	}
	c.subMu.RUnlock()

	return map[string]any{
		"pending_requests":  pendingCount,
		"subscribed_topics": subscribedTopics,
	}
}
