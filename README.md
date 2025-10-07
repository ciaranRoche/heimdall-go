# Heimdall Go

A production-ready messaging abstraction library for Go, providing unified interfaces for Apache Kafka, RabbitMQ, and other messaging providers.

## Features

- 🔌 **Multi-Provider Support**: Kafka, RabbitMQ, with extensible provider registry
- 🎯 **Smart Routing**: Topic-based routing with pattern matching and response handling
- 📊 **OpenTelemetry Integration**: Built-in metrics, tracing, and structured logging
- 🧪 **Testable**: Testcontainers integration for realistic integration tests
- ⚡ **Performance**: Zero-allocation paths, connection pooling, efficient serialization
- 🛡️ **Production Ready**: Graceful shutdown, health checks, error handling

## Installation

Install the latest version:

```bash
go get github.com/ciaranRoche/heimdall-go@latest
```

Or install a specific version:

```bash
go get github.com/ciaranRoche/heimdall-go@v0.2.0
```

Add to your `go.mod`:

```go
require github.com/ciaranRoche/heimdall-go v0.2.0
```

**Releases**: See [GitHub Releases](https://github.com/ciaranRoche/heimdall-go/releases) for version history and changelogs.

## Quick Start

```go
package main

import (
    "context"
    "log"

    "github.com/ciaranRoche/heimdall-go"
    _ "github.com/ciaranRoche/heimdall-go/provider/kafka"
    _ "github.com/ciaranRoche/heimdall-go/provider/rabbitmq"
)

func main() {
    // Create Heimdall instance with Kafka provider
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

    // Publish a message
    ctx := context.Background()
    err = h.Publish(ctx, "my-topic", []byte("Hello, Heimdall!"))
    if err != nil {
        log.Fatal(err)
    }

    // Subscribe to messages
    err = h.Subscribe(ctx, "my-topic", func(ctx context.Context, msg *heimdall.Message) error {
        log.Printf("Received: %s", string(msg.Data))
        return nil
    })
    if err != nil {
        log.Fatal(err)
    }
}
```

## Provider Configuration

### Kafka

```go
h, err := heimdall.New(
    heimdall.WithProvider("kafka"),
    heimdall.WithConfig(map[string]interface{}{
        "bootstrap_servers": []string{"localhost:9092"},
        "consumer_group": "my-consumer-group",
        "security_protocol": "SASL_SSL",
        "sasl_username": "user",
        "sasl_password": "pass",
    }),
)
```

### RabbitMQ

```go
h, err := heimdall.New(
    heimdall.WithProvider("rabbitmq"),
    heimdall.WithConfig(map[string]interface{}{
        "url": "amqp://guest:guest@localhost:5672/",
        "exchange": "my-exchange",
        "exchange_type": "topic",
    }),
)
```

## Advanced Features

### Routing Engine

```go
// Configure routing with response handling
h, err := heimdall.New(
    heimdall.WithProvider("kafka"),
    heimdall.WithRouting(heimdall.RoutingConfig{
        Routes: []heimdall.Route{
            {
                Pattern: "order.created",
                Target: "orders.topic",
                ResponseHandler: handleOrderResponse,
            },
        },
    }),
)
```

### OpenTelemetry Integration

```go
import "go.opentelemetry.io/otel"

h, err := heimdall.New(
    heimdall.WithProvider("kafka"),
    heimdall.WithObservability(heimdall.ObservabilityConfig{
        Metrics: true,
        Tracing: true,
        Logger: yourLogger,
    }),
)
```

## Documentation

- [Architecture Overview](docs/architecture.md)
- [Provider Guide](docs/providers.md)
- [Routing Engine](docs/routing.md)
- [Observability](docs/observability.md)
- [Migration Guide](docs/migration.md)

## Examples

See [examples/](examples/) directory for complete working examples:

- [Basic Publisher/Consumer](examples/basic/)
- [Routing with Responses](examples/routing/)
- [Multi-Provider Setup](examples/multi-provider/)
- [Observability Integration](examples/observability/)

## Project Status

**Status**: Active Development
**Version**: v0.1.0 (Alpha)

This library is extracted from the [rh-trex](https://github.com/ciaranRoche/rh-trex) project and is being prepared for production use.

## Contributing

Contributions welcome! Please see [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

## License

Apache License 2.0 - see [LICENSE](LICENSE)
