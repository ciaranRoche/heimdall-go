# Topic/Queue Management

Heimdall provides topic and queue management capabilities for supported messaging providers. This allows you to programmatically create, delete, and configure topics/queues without using provider-specific tools.

## Features

- **Auto-creation of topics/queues**: Create topics and queues programmatically
- **Exchange declaration (RabbitMQ)**: Declare and configure exchanges
- **Topic configuration**: Configure partitions, replication, retention, etc.
- **Topic inspection**: Check if topics exist and list all topics
- **Provider-agnostic API**: Same interface for Kafka and RabbitMQ

## Provider Support

| Provider | Create | Delete | Exists | List | Update | Notes |
|----------|--------|--------|--------|------|--------|-------|
| Kafka | ✅ | ✅ | ✅ | ✅ | ✅ | Full support via Cluster Admin API |
| RabbitMQ | ✅ | ✅ | ✅ | ❌ | ❌ | List/Update require Management HTTP API |

## Quick Start

### Check if Provider Supports Topic Management

```go
package main

import (
    "context"
    "log"

    "github.com/ciaranRoche/heimdall-go"
)

func main() {
    h, err := heimdall.New(
        heimdall.WithProvider("kafka"),
        heimdall.WithConfig(map[string]interface{}{
            "bootstrap_servers": []string{"localhost:9092"},
        }),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer h.Close()

    // Check if provider supports topic management
    if h.SupportsTopicManagement() {
        log.Println("Provider supports topic management")
    } else {
        log.Println("Provider does not support topic management")
    }
}
```

### Creating Topics

#### Kafka Example

```go
package main

import (
    "context"
    "log"

    "github.com/ciaranRoche/heimdall-go"
    "github.com/ciaranRoche/heimdall-go/provider"
)

func main() {
    h, err := heimdall.New(
        heimdall.WithProvider("kafka"),
        heimdall.WithConfig(map[string]interface{}{
            "bootstrap_servers": []string{"localhost:9092"},
        }),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer h.Close()

    ctx := context.Background()

    // Create a Kafka topic with 3 partitions and replication factor of 2
    err = h.CreateTopic(ctx, &provider.TopicConfig{
        Name:              "orders.created",
        Partitions:        3,
        ReplicationFactor: 2,
        ConfigEntries: map[string]string{
            "retention.ms":     "86400000",  // 24 hours
            "cleanup.policy":   "delete",
            "compression.type": "gzip",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Topic created successfully")
}
```

#### RabbitMQ Example

```go
package main

import (
    "context"
    "log"

    "github.com/ciaranRoche/heimdall-go"
    "github.com/ciaranRoche/heimdall-go/provider"
)

func main() {
    h, err := heimdall.New(
        heimdall.WithProvider("rabbitmq"),
        heimdall.WithConfig(map[string]interface{}{
            "url": "amqp://guest:guest@localhost:5672/",
        }),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer h.Close()

    ctx := context.Background()

    // Create a RabbitMQ queue with exchange
    err = h.CreateTopic(ctx, &provider.TopicConfig{
        Name:         "orders-queue",
        ExchangeType: "topic",
        Durable:      true,
        AutoDelete:   false,
        Bindings: map[string]string{
            "orders.#": "orders-exchange",
        },
        Arguments: map[string]interface{}{
            "x-message-ttl": 86400000, // 24 hours
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Queue created successfully")
}
```

### Checking if Topic Exists

```go
exists, err := h.TopicExists(ctx, "orders.created")
if err != nil {
    log.Fatal(err)
}

if exists {
    log.Println("Topic exists")
} else {
    log.Println("Topic does not exist")
}
```

### Listing Topics

```go
topics, err := h.ListTopics(ctx)
if err != nil {
    log.Fatal(err)
}

log.Printf("Found %d topics:\n", len(topics))
for _, topic := range topics {
    log.Printf("  - %s\n", topic)
}
```

### Updating Topic Configuration

```go
// Update Kafka topic configuration
err = h.UpdateTopicConfig(ctx, "orders.created", &provider.TopicConfig{
    Name: "orders.created",
    ConfigEntries: map[string]string{
        "retention.ms": "172800000", // Increase to 48 hours
    },
})
if err != nil {
    log.Fatal(err)
}

log.Println("Topic configuration updated")
```

### Deleting Topics

```go
err = h.DeleteTopic(ctx, "orders.created")
if err != nil {
    log.Fatal(err)
}

log.Println("Topic deleted successfully")
```

## TopicConfig Reference

The `TopicConfig` struct contains configuration for creating and updating topics:

```go
type TopicConfig struct {
    // Name is the topic/queue name (required)
    Name string

    // Kafka-specific fields
    Partitions        int32             // Number of partitions
    ReplicationFactor int16             // Replication factor
    ConfigEntries     map[string]string // Kafka config entries

    // RabbitMQ-specific fields
    ExchangeType string                   // "direct", "topic", "fanout", "headers"
    Durable      bool                     // Survive broker restart
    AutoDelete   bool                     // Delete when unused
    Bindings     map[string]string        // Routing key -> exchange bindings
    Arguments    map[string]interface{}   // Additional arguments
}
```

### Kafka Configuration Entries

Common Kafka `ConfigEntries`:

| Key | Description | Example |
|-----|-------------|---------|
| `retention.ms` | Message retention time in milliseconds | `"86400000"` (24 hours) |
| `cleanup.policy` | Cleanup policy: "delete" or "compact" | `"delete"` |
| `compression.type` | Compression: "none", "gzip", "snappy", "lz4", "zstd" | `"gzip"` |
| `max.message.bytes` | Maximum message size in bytes | `"1048576"` (1MB) |
| `segment.ms` | Segment file time before rolling | `"604800000"` (7 days) |
| `min.insync.replicas` | Minimum in-sync replicas | `"2"` |

See [Kafka Topic Configuration](https://kafka.apache.org/documentation/#topicconfigs) for all options.

### RabbitMQ Arguments

Common RabbitMQ `Arguments`:

| Key | Description | Example |
|-----|-------------|---------|
| `x-message-ttl` | Message TTL in milliseconds | `86400000` (24 hours) |
| `x-max-length` | Maximum queue length | `10000` |
| `x-max-length-bytes` | Maximum queue size in bytes | `1048576` (1MB) |
| `x-overflow` | Overflow behavior: "drop-head", "reject-publish" | `"drop-head"` |
| `x-dead-letter-exchange` | Dead letter exchange name | `"dlx"` |
| `x-dead-letter-routing-key` | Dead letter routing key | `"dlq"` |

See [RabbitMQ Queue Arguments](https://www.rabbitmq.com/queues.html) for all options.

## Error Handling

Topic management operations can return specific errors:

```go
import "github.com/ciaranRoche/heimdall-go/provider"

err := h.CreateTopic(ctx, config)
if err != nil {
    switch err {
    case provider.ErrTopicManagementNotSupported:
        log.Println("Provider doesn't support topic management")
    case provider.ErrTopicAlreadyExists:
        log.Println("Topic already exists")
    case provider.ErrTopicNotFound:
        log.Println("Topic not found")
    case provider.ErrInvalidTopicName:
        log.Println("Invalid topic name")
    default:
        log.Printf("Error: %v", err)
    }
}
```

## Complete Example: Topic Lifecycle

```go
package main

import (
    "context"
    "log"
    "time"

    "github.com/ciaranRoche/heimdall-go"
    "github.com/ciaranRoche/heimdall-go/provider"
)

func main() {
    // Create Heimdall client
    h, err := heimdall.New(
        heimdall.WithProvider("kafka"),
        heimdall.WithConfig(map[string]interface{}{
            "bootstrap_servers": []string{"localhost:9092"},
        }),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer h.Close()

    ctx := context.Background()
    topicName := "demo.lifecycle"

    // 1. Check if topic management is supported
    if !h.SupportsTopicManagement() {
        log.Fatal("Provider doesn't support topic management")
    }

    // 2. Check if topic already exists
    exists, err := h.TopicExists(ctx, topicName)
    if err != nil {
        log.Fatal(err)
    }

    if exists {
        log.Printf("Topic %s already exists, deleting...\n", topicName)
        if err := h.DeleteTopic(ctx, topicName); err != nil {
            log.Fatal(err)
        }
        // Wait for topic deletion to propagate
        time.Sleep(2 * time.Second)
    }

    // 3. Create topic
    log.Printf("Creating topic %s...\n", topicName)
    err = h.CreateTopic(ctx, &provider.TopicConfig{
        Name:              topicName,
        Partitions:        3,
        ReplicationFactor: 1,
        ConfigEntries: map[string]string{
            "retention.ms":   "3600000", // 1 hour
            "cleanup.policy": "delete",
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // 4. Verify topic exists
    exists, err = h.TopicExists(ctx, topicName)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Topic exists: %v\n", exists)

    // 5. List all topics
    topics, err := h.ListTopics(ctx)
    if err != nil {
        log.Fatal(err)
    }
    log.Printf("Found %d topics\n", len(topics))

    // 6. Update topic configuration
    log.Println("Updating topic configuration...")
    err = h.UpdateTopicConfig(ctx, topicName, &provider.TopicConfig{
        Name: topicName,
        ConfigEntries: map[string]string{
            "retention.ms": "7200000", // Increase to 2 hours
        },
    })
    if err != nil {
        log.Fatal(err)
    }

    // 7. Use the topic for messaging
    log.Println("Publishing test message...")
    err = h.Publish(ctx, topicName, []byte("Hello, Heimdall!"))
    if err != nil {
        log.Fatal(err)
    }

    // 8. Clean up: delete topic
    log.Printf("Deleting topic %s...\n", topicName)
    err = h.DeleteTopic(ctx, topicName)
    if err != nil {
        log.Fatal(err)
    }

    log.Println("Topic lifecycle demo completed")
}
```

## Best Practices

### 1. Check Provider Support

Always check if the provider supports topic management before calling topic management methods:

```go
if !h.SupportsTopicManagement() {
    return fmt.Errorf("provider doesn't support topic management")
}
```

### 2. Handle Errors Gracefully

Handle provider-specific errors appropriately:

```go
err := h.CreateTopic(ctx, config)
if err != nil {
    if err == provider.ErrTopicAlreadyExists {
        log.Println("Topic already exists, continuing...")
        return nil
    }
    return fmt.Errorf("failed to create topic: %w", err)
}
```

### 3. Use Idempotent Operations

Check if a topic exists before creating it:

```go
exists, err := h.TopicExists(ctx, topicName)
if err != nil {
    return err
}

if !exists {
    err = h.CreateTopic(ctx, config)
    if err != nil && err != provider.ErrTopicAlreadyExists {
        return err
    }
}
```

### 4. Set Appropriate Defaults

For Kafka, use sensible defaults for partitions and replication:

```go
partitions := int32(3)    // 3 partitions for parallelism
replication := int16(2)   // Replication factor of 2 for durability

config := &provider.TopicConfig{
    Name:              topicName,
    Partitions:        partitions,
    ReplicationFactor: replication,
}
```

### 5. Configure Retention Policies

Set appropriate retention based on your use case:

```go
// Short-lived events (1 hour)
ConfigEntries: map[string]string{
    "retention.ms": "3600000",
}

// Long-term storage (7 days)
ConfigEntries: map[string]string{
    "retention.ms": "604800000",
}

// Infinite retention
ConfigEntries: map[string]string{
    "retention.ms": "-1",
}
```

### 6. Use Durable Queues in Production

For RabbitMQ, always use durable queues in production:

```go
config := &provider.TopicConfig{
    Name:         queueName,
    Durable:      true,      // Survive broker restart
    AutoDelete:   false,     // Don't auto-delete
    ExchangeType: "topic",
}
```

## Integration with Application Initialization

Topic management is commonly used during application startup:

```go
func initializeTopics(h *heimdall.Heimdall) error {
    ctx := context.Background()

    if !h.SupportsTopicManagement() {
        log.Println("Skipping topic initialization: not supported")
        return nil
    }

    topics := []provider.TopicConfig{
        {
            Name:              "orders.created",
            Partitions:        3,
            ReplicationFactor: 2,
            ConfigEntries: map[string]string{
                "retention.ms": "86400000",
            },
        },
        {
            Name:              "orders.updated",
            Partitions:        3,
            ReplicationFactor: 2,
            ConfigEntries: map[string]string{
                "retention.ms": "86400000",
            },
        },
    }

    for _, config := range topics {
        exists, err := h.TopicExists(ctx, config.Name)
        if err != nil {
            return fmt.Errorf("failed to check topic %s: %w", config.Name, err)
        }

        if !exists {
            log.Printf("Creating topic: %s\n", config.Name)
            err = h.CreateTopic(ctx, &config)
            if err != nil && err != provider.ErrTopicAlreadyExists {
                return fmt.Errorf("failed to create topic %s: %w", config.Name, err)
            }
        }
    }

    return nil
}

func main() {
    h, err := heimdall.New(
        heimdall.WithProvider("kafka"),
        heimdall.WithConfig(map[string]interface{}{
            "bootstrap_servers": []string{"localhost:9092"},
        }),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer h.Close()

    // Initialize topics on startup
    if err := initializeTopics(h); err != nil {
        log.Fatal(err)
    }

    // Application continues...
}
```

## Limitations

### RabbitMQ

- **ListTopics**: Requires RabbitMQ Management HTTP API (not available via AMQP)
- **UpdateTopicConfig**: RabbitMQ doesn't support updating queue properties after creation. Delete and recreate instead.

### Kafka

- **Topic Deletion**: May not be immediate due to asynchronous deletion
- **Partition Count**: Cannot be decreased, only increased
- **Replication Factor**: Cannot be changed after topic creation

## See Also

- [Kafka Topic Configuration](https://kafka.apache.org/documentation/#topicconfigs)
- [RabbitMQ Queue Documentation](https://www.rabbitmq.com/queues.html)
- [RabbitMQ Exchange Types](https://www.rabbitmq.com/tutorials/amqp-concepts.html#exchanges)
