# Request/Reply Pattern

The request/reply package provides a synchronous request-response pattern on top of Heimdall's asynchronous messaging system.

## Features

- **Synchronous Requests**: Send a request and wait for the response
- **Async Requests**: Send a request and handle the response in a callback
- **Automatic Correlation ID Tracking**: Automatically tracks which response belongs to which request
- **Auto-Subscription**: Automatically subscribes to response topics
- **Timeout Support**: Configure request timeouts with automatic cleanup
- **Concurrent Requests**: Handle multiple concurrent request/reply flows safely

## Quick Start

### Synchronous Request/Reply

```go
package main

import (
    "context"
    "fmt"
    "time"

    "github.com/ciaranRoche/heimdall-go"
    "github.com/ciaranRoche/heimdall-go/requestreply"
)

func main() {
    // Create Heimdall client
    h, _ := heimdall.New(
        heimdall.WithProvider("kafka"),
        heimdall.WithConfig(map[string]any{
            "bootstrap_servers": []string{"localhost:9092"},
            "consumer_group":    "my-group",
        }),
    )
    defer h.Close()

    // Create request/reply client
    client := requestreply.NewClient(h)
    defer client.Close()

    // Send request and wait for response
    req := &requestreply.Request{
        Topic:         "scoring.requests",
        Data:          []byte(`{"entity_id": "123"}`),
        ResponseTopic: "scoring.responses",
        Timeout:       30 * time.Second,
    }

    response, err := client.Request(context.Background(), req)
    if err != nil {
        panic(err)
    }

    fmt.Printf("Received response: %s\n", response.Data)
}
```

### Asynchronous Request/Reply

```go
// Send request with callback handler
req := &requestreply.Request{
    Topic:         "enrichment.requests",
    Data:          []byte(`{"entity_id": "456"}`),
    ResponseTopic: "enrichment.responses",
    Timeout:       30 * time.Second,
}

handler := func(ctx context.Context, msg *heimdall.Message) error {
    fmt.Printf("Received response: %s\n", msg.Data)
    return nil
}

err := client.RequestAsync(context.Background(), req, handler)
if err != nil {
    panic(err)
}

// Handler will be called when response arrives
```

## Request Options

### Custom Correlation ID

```go
req := &requestreply.Request{
    Topic:         "requests.topic",
    Data:          []byte("data"),
    ResponseTopic: "responses.topic",
    CorrelationID: "custom-correlation-123", // Custom ID
    Timeout:       30 * time.Second,
}
```

If not specified, a UUID will be automatically generated.

### Request Headers

```go
req := &requestreply.Request{
    Topic:         "requests.topic",
    Data:          []byte("data"),
    ResponseTopic: "responses.topic",
    Headers: map[string]any{
        "x-user-id": "user123",
        "x-priority": "high",
    },
    Timeout: 30 * time.Second,
}
```

Headers are automatically augmented with:
- `correlation_id`: The request correlation ID
- `response_topic`: The topic to publish the response to

### Timeout Configuration

```go
req := &requestreply.Request{
    Topic:         "requests.topic",
    Data:          []byte("data"),
    ResponseTopic: "responses.topic",
    Timeout:       5 * time.Second,  // Custom timeout
}
```

Default timeout is 30 seconds if not specified.

## Response Handling

### Synchronous Response

```go
response, err := client.Request(ctx, req)
if err != nil {
    // Timeout, context cancellation, or publish error
    return err
}

// Check for error in response
if response.Error != nil {
    // Response contained an error (from "error" header)
    return response.Error
}

// Process response data
fmt.Printf("Response: %s\n", response.Data)
fmt.Printf("Correlation ID: %s\n", response.CorrelationID)
fmt.Printf("Headers: %v\n", response.Headers)
```

### Async Response Handler

```go
handler := func(ctx context.Context, msg *heimdall.Message) error {
    if msg.Data == nil {
        // Timeout or error
        return fmt.Errorf("request timed out")
    }

    // Process response
    fmt.Printf("Received: %s\n", msg.Data)
    return nil
}

err := client.RequestAsync(ctx, req, handler)
```

### Error Responses

If the response message contains an `error` header, it will be automatically converted to an error:

```go
// Responder sends error
responder.Publish(ctx, responseTopic, []byte("error details"),
    heimdall.WithHeaders(map[string]any{
        "correlation_id": correlationID,
        "error": "validation failed",  // Will be converted to error
    }),
)

// Requestor receives error
response, err := client.Request(ctx, req)
// response.Error will contain "validation failed"
```

## Auto-Subscription

The request/reply client automatically subscribes to response topics the first time they're used:

```go
// First request to this response topic - subscription created automatically
req1 := &requestreply.Request{
    ResponseTopic: "responses.topic",
    // ...
}
client.Request(ctx, req1)

// Second request to same topic - reuses existing subscription
req2 := &requestreply.Request{
    ResponseTopic: "responses.topic",
    // ...
}
client.Request(ctx, req2)
```

Subscriptions persist until `client.Close()` is called.

## Concurrent Requests

The client is safe for concurrent use:

```go
var wg sync.WaitGroup

for i := 0; i < 10; i++ {
    wg.Add(1)
    go func(id int) {
        defer wg.Done()

        req := &requestreply.Request{
            Topic:         "requests.topic",
            Data:          []byte(fmt.Sprintf("request-%d", id)),
            ResponseTopic: "responses.topic",
            Timeout:       30 * time.Second,
        }

        response, err := client.Request(context.Background(), req)
        if err != nil {
            log.Printf("Request %d failed: %v", id, err)
            return
        }

        log.Printf("Request %d succeeded: %s", id, response.Data)
    }(i)
}

wg.Wait()
```

## Integration Example

Complete example showing requester and responder:

### Responder

```go
package main

import (
    "context"
    "encoding/json"

    "github.com/ciaranRoche/heimdall-go"
)

type Request struct {
    EntityID string `json:"entity_id"`
}

type Response struct {
    Result string `json:"result"`
}

func main() {
    h, _ := heimdall.New(
        heimdall.WithProvider("kafka"),
        heimdall.WithConfig(map[string]any{
            "bootstrap_servers": []string{"localhost:9092"},
            "consumer_group":    "responder-group",
        }),
    )
    defer h.Close()

    // Subscribe to requests
    _, _ = h.Subscribe(context.Background(), "requests.topic",
        func(ctx context.Context, msg *heimdall.Message) error {
            // Parse request
            var req Request
            json.Unmarshal(msg.Data, &req)

            // Process request
            resp := Response{
                Result: fmt.Sprintf("Processed %s", req.EntityID),
            }
            respData, _ := json.Marshal(resp)

            // Get response topic and correlation ID from headers
            responseTopic := msg.Headers["response_topic"].(string)
            correlationID := msg.CorrelationID

            // Publish response
            return h.Publish(ctx, responseTopic, respData,
                heimdall.WithCorrelationID(correlationID),
            )
        },
    )

    select {} // Wait forever
}
```

### Requester

```go
package main

import (
    "context"
    "encoding/json"
    "fmt"
    "time"

    "github.com/ciaranRoche/heimdall-go"
    "github.com/ciaranRoche/heimdall-go/requestreply"
)

type Request struct {
    EntityID string `json:"entity_id"`
}

type Response struct {
    Result string `json:"result"`
}

func main() {
    h, _ := heimdall.New(
        heimdall.WithProvider("kafka"),
        heimdall.WithConfig(map[string]any{
            "bootstrap_servers": []string{"localhost:9092"},
            "consumer_group":    "requester-group",
        }),
    )
    defer h.Close()

    client := requestreply.NewClient(h)
    defer client.Close()

    // Create request
    reqData, _ := json.Marshal(Request{EntityID: "123"})

    req := &requestreply.Request{
        Topic:         "requests.topic",
        Data:          reqData,
        ResponseTopic: "responses.topic",
        Timeout:       30 * time.Second,
    }

    // Send and wait for response
    response, err := client.Request(context.Background(), req)
    if err != nil {
        panic(err)
    }

    // Parse response
    var resp Response
    json.Unmarshal(response.Data, &resp)

    fmt.Printf("Result: %s\n", resp.Result)
}
```

## Statistics

Get request/reply statistics:

```go
stats := client.Stats()
// Returns:
// {
//   "pending_requests": 3,
//   "subscribed_topics": ["responses.topic1", "responses.topic2"]
// }

// Check pending requests
pending := client.PendingRequests()
fmt.Printf("Pending requests: %d\n", pending)
```

## Cleanup

Always close the client when done:

```go
client := requestreply.NewClient(h)
defer client.Close()

// Or explicitly
err := client.Close()
if err != nil {
    log.Printf("Error closing client: %v", err)
}
```

Closing the client:
- Cancels all pending requests
- Unsubscribes from all response topics
- Cleans up internal state

## Best Practices

1. **Set appropriate timeouts** - Default is 30s, adjust based on your use case
2. **Reuse clients** - Create one client per Heimdall instance
3. **Always close** - Use `defer client.Close()` to ensure cleanup
4. **Handle timeouts** - Implement retry logic for timeout scenarios
5. **Use context** - Pass appropriate context for cancellation
6. **Unique response topics** - Use different response topics for different request types

## Error Handling

Common error scenarios:

```go
response, err := client.Request(ctx, req)
if err != nil {
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        // Request timed out
        return fmt.Errorf("request timed out: %w", err)

    case errors.Is(err, context.Canceled):
        // Context was canceled
        return fmt.Errorf("request canceled: %w", err)

    default:
        // Publish error or other failure
        return fmt.Errorf("request failed: %w", err)
    }
}

// Check response error
if response.Error != nil {
    return fmt.Errorf("response error: %w", response.Error)
}
```

## Testing

The request/reply client accepts any type that implements the `MessagingClient` interface, making it easy to test:

```go
type mockClient struct {
    // Mock implementation
}

func (m *mockClient) Publish(ctx context.Context, topic string, data []byte, opts ...heimdall.PublishOption) error {
    // Mock publish
    return nil
}

func (m *mockClient) Subscribe(ctx context.Context, topic string, handler heimdall.MessageHandler) (context.CancelFunc, error) {
    // Mock subscribe
    return func() {}, nil
}

// Use in tests
client := requestreply.NewClient(&mockClient{})
```
