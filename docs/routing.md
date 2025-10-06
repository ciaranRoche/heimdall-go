# Routing Engine

The routing engine provides CEL-based conditional message routing with support for entity data providers and multi-destination publishing.

## Features

- **CEL Expression Support**: Use Google's Common Expression Language for powerful routing conditions
- **Entity Data Providers**: Access entity data in routing decisions
- **Multi-Destination Routing**: Route messages to multiple destinations based on rules
- **Priority-Based Evaluation**: Control rule evaluation order with priorities
- **Stop-on-Match**: Optionally stop evaluation after first matching rule

## Quick Start

```go
package main

import (
    "context"
    "github.com/ciaranRoche/heimdall-go/routing"
)

func main() {
    // Create routing engine
    engine, err := routing.NewEngine(&routing.Config{
        StopOnFirstMatch: false,
        EnableMetrics: true,
    })
    if err != nil {
        panic(err)
    }

    // Add routing rule
    err = engine.AddRule(&routing.Rule{
        Name:      "route-active-dinosaurs",
        Condition: "entity.status == 'active' && entity.species == 'T-Rex'",
        Priority:  10,
        Destinations: []routing.Destination{
            {Topic: "dinosaur.scoring.requests", Provider: "kafka"},
        },
    })
    if err != nil {
        panic(err)
    }

    // Route a message
    msg := &routing.Message{
        Topic: "dinosaur.events",
        Data:  []byte(`{"id": "123"}`),
        Entity: map[string]any{
            "status":  "active",
            "species": "T-Rex",
        },
    }

    destinations, err := engine.Route(context.Background(), msg)
    if err != nil {
        panic(err)
    }

    // destinations contains all matching routes
}
```

## CEL Expressions

The routing engine provides these variables in CEL expressions:

- `message.topic` - Source topic/queue name
- `message.data` - Message payload (bytes)
- `headers` - Message headers (map[string]any)
- `entity` - Entity data (map[string]any)
- `event` - Event metadata (map[string]any)

### Example Expressions

```javascript
// Match by entity status
entity.status == 'active'

// Match by species and status
entity.species == 'T-Rex' && entity.status == 'validated'

// Match by message topic
message.topic.startsWith('dinosaur.')

// Match by header
headers.priority == 'high'

// Complex conditions
entity.status == 'active' &&
  (entity.species == 'T-Rex' || entity.species == 'Velociraptor') &&
  headers.region in ['us-east', 'us-west']
```

## Entity Data Providers

Register entity data providers to access entity information in routing decisions:

```go
type DinosaurProvider struct {
    db *sql.DB
}

func (p *DinosaurProvider) GetEntity(ctx context.Context, entityID, entityType string) (map[string]any, error) {
    // Fetch from database
    var status, species string
    err := p.db.QueryRow("SELECT status, species FROM dinosaurs WHERE id = ?", entityID).
        Scan(&status, &species)
    if err != nil {
        return nil, err
    }

    return map[string]any{
        "status":  status,
        "species": species,
    }, nil
}

func (p *DinosaurProvider) EntityType() string {
    return "dinosaur"
}

// Register the provider
engine.RegisterEntityProvider("dinosaur", &DinosaurProvider{db: db})
```

The engine automatically fetches entity data when:
- `msg.Entity` is nil
- `msg.Event` contains `entity_id` and `entity_type`
- A provider is registered for the entity type

## Rule Priority

Rules are evaluated in priority order (highest first):

```go
engine.AddRule(&routing.Rule{
    Name:     "high-priority",
    Priority: 100,
    // ... evaluated first
})

engine.AddRule(&routing.Rule{
    Name:     "low-priority",
    Priority: 10,
    // ... evaluated later
})
```

## Stop on Match

Stop evaluation after first match:

```go
// Global configuration
engine, _ := routing.NewEngine(&routing.Config{
    StopOnFirstMatch: true,
})

// Per-rule configuration
engine.AddRule(&routing.Rule{
    Name:        "exclusive-rule",
    StopOnMatch: true,  // Stops evaluation if this rule matches
    // ...
})
```

## Routing Statistics

Get routing engine statistics:

```go
stats := engine.Stats()
// Returns:
// {
//   "total_rules": 5,
//   "entity_providers": 2,
//   "provider_types": ["dinosaur", "habitat"],
//   "stop_on_first": false,
//   "metrics_enabled": true,
//   "default_provider": "kafka"
// }
```

## Integration with Heimdall

Use routing engine with Heimdall messaging:

```go
// Create Heimdall client
heimdall, _ := heimdall.New(&heimdall.Config{
    Provider: "kafka",
    Config: map[string]any{
        "bootstrap_servers": []string{"localhost:9092"},
    },
})

// Create routing engine
router, _ := routing.NewEngine(config)

// Subscribe to source topic
heimdall.Subscribe("dinosaur.events", func(msg heimdall.Message) error {
    // Build routing message
    routeMsg := &routing.Message{
        Topic:   msg.Topic,
        Data:    msg.Data,
        Headers: msg.Headers,
        Event:   msg.Metadata,
    }

    // Get destinations
    destinations, err := router.Route(context.Background(), routeMsg)
    if err != nil {
        return err
    }

    // Publish to all destinations
    for _, dest := range destinations {
        // Merge headers
        headers := make(map[string]any)
        for k, v := range msg.Headers {
            headers[k] = v
        }
        for k, v := range dest.Headers {
            headers[k] = v
        }

        err = heimdall.Publish(dest.Topic, msg.Data,
            heimdall.WithHeaders(headers),
            heimdall.WithCorrelationID(msg.CorrelationID),
        )
        if err != nil {
            return err
        }
    }

    return nil
})
```

## Testing

The routing package includes comprehensive tests:

```bash
# Run routing tests
go test -v ./routing/

# Run with race detection
go test -race ./routing/

# Run all tests
make test
```

## CEL Documentation

For more information on CEL expressions, see:
- [CEL Language Definition](https://github.com/google/cel-spec)
- [CEL Go Implementation](https://github.com/google/cel-go)
