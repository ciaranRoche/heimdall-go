# Routing Engine

The routing engine provides CEL-based conditional message routing with support for entity data providers and multi-destination publishing.

## Features

- **CEL Expression Support**: Use Google's Common Expression Language for powerful routing conditions
- **Entity Data Providers**: Access entity data in routing decisions
- **Multi-Destination Routing**: Route messages to multiple destinations based on rules
- **Priority-Based Evaluation**: Control rule evaluation order with priorities
- **Stop-on-Match**: Optionally stop evaluation after first matching rule

## Quick Start

### Programmatic Configuration

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

### YAML Configuration

Load routing rules from a YAML configuration file:

```go
package main

import (
    "context"
    "github.com/ciaranRoche/heimdall-go/routing"
)

func main() {
    // Load config from YAML file
    config, err := routing.LoadConfigFromFile("routing.yaml")
    if err != nil {
        panic(err)
    }

    // Create engine with loaded config
    engine, err := routing.NewEngine(config)
    if err != nil {
        panic(err)
    }

    // Route messages
    msg := &routing.Message{
        Topic: "dinosaur.events",
        Entity: map[string]any{
            "status":  "active",
            "species": "T-Rex",
        },
    }

    destinations, _ := engine.Route(context.Background(), msg)
}
```

**routing.yaml:**
```yaml
global:
  stop_on_first_match: false
  enable_metrics: true
  default_provider: "kafka"

entities:
  dinosaur:
    routing_rules:
      - name: "route-active-dinosaurs"
        priority: 10
        condition:
          expression: 'entity.status == "active" && entity.species == "T-Rex"'
        publish:
          - topic: "dinosaur.scoring.requests"
            provider: "kafka"
```

See [examples/config.yaml](../examples/config.yaml) for a complete configuration example.

## YAML Configuration Reference

### Global Settings

```yaml
global:
  # Stop evaluation after first matching rule (default: false)
  stop_on_first_match: false

  # Enable routing metrics and logging (default: false)
  enable_metrics: true

  # Default provider when not specified in destinations
  default_provider: "kafka"
```

### Entity-Specific Configuration

Each entity can have its own routing rules and override global settings:

```yaml
entities:
  dinosaur:
    # Override global default provider for this entity
    default_provider: "rabbitmq"

    # Override stop_on_first_match for this entity
    stop_on_first_match: true

    routing_rules:
      - name: "rule-name"
        priority: 10
        condition:
          expression: 'entity.status == "active"'
        publish:
          - topic: "destination.topic"
```

### Provider Override Precedence

Providers are resolved in this order (highest precedence first):

1. **Destination-level** - Explicit provider in `publish.provider`
2. **Entity-level** - Entity's `default_provider`
3. **Global-level** - Global `default_provider`

```yaml
global:
  default_provider: "kafka"  # Used if no other provider specified

entities:
  dinosaur:
    default_provider: "rabbitmq"  # Overrides global for dinosaur

    routing_rules:
      - name: "example"
        condition:
          expression: "true"
        publish:
          - topic: "topic1"
            # Uses entity default: rabbitmq

          - topic: "topic2"
            provider: "nats"
            # Uses explicit provider: nats
```

### Routing Rules

Each routing rule must have:
- **name**: Unique identifier
- **condition**: CEL expression that evaluates to boolean
- **publish**: Array of destinations

Optional fields:
- **priority**: Evaluation order (higher = first, default: 0)
- **stop_on_match**: Stop evaluation if this rule matches (default: false)

```yaml
routing_rules:
  - name: "high-priority-rule"
    priority: 100
    stop_on_match: true
    condition:
      expression: 'entity.critical == true'
    publish:
      - topic: "alerts.critical"
        provider: "kafka"
        headers:
          x-priority: "high"
          x-alert: "true"
```

### Destinations

Each destination configuration supports:

```yaml
publish:
  - topic: "destination.topic"        # Required: destination topic/queue
    provider: "kafka"                 # Optional: messaging provider
    transform: "json_to_proto"        # Optional: transformation to apply
    headers:                          # Optional: additional headers
      x-source: "heimdall"
      x-priority: "high"
```

### Multi-Destination Publishing

A single rule can publish to multiple destinations (fan-out):

```yaml
routing_rules:
  - name: "fan-out-critical"
    condition:
      expression: 'entity.critical == true'
    publish:
      - topic: "alerts.critical"
        provider: "kafka"
      - topic: "alerts.backup"
        provider: "rabbitmq"
      - topic: "monitoring.events"
        provider: "nats"
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

## Callbacks

The routing engine supports lifecycle callbacks for monitoring and integration:

```go
callbacks := &routing.Callbacks{
    OnBeforeRoute: func(ctx context.Context, msg *routing.Message) error {
        log.Printf("Routing message from topic: %s", msg.Topic)
        return nil
    },
    OnAfterRoute: func(ctx context.Context, msg *routing.Message, destinations []routing.Destination, err error) {
        log.Printf("Routed to %d destinations", len(destinations))
    },
    OnRuleMatch: func(ctx context.Context, msg *routing.Message, rule *routing.Rule, destinations []routing.Destination) {
        log.Printf("Rule matched: %s -> %v", rule.Name, destinations)
    },
    OnEntityFetch: func(ctx context.Context, entityID, entityType string, data map[string]any, err error) {
        if err != nil {
            log.Printf("Failed to fetch %s/%s: %v", entityType, entityID, err)
        } else {
            log.Printf("Fetched %s/%s: %v", entityType, entityID, data)
        }
    },
    OnRoutingError: func(ctx context.Context, msg *routing.Message, err error) {
        log.Printf("Routing error: %v", err)
    },
}

config := &routing.Config{
    Rules:     rules,
    Callbacks: callbacks,
}
```

### Callback Types

**OnBeforeRoute**
- Called before routing evaluation starts
- Can return error to abort routing
- Use for validation, preprocessing, or access control

**OnAfterRoute**
- Called after routing evaluation completes
- Receives destinations and any error
- Use for logging, metrics, or cleanup

**OnRuleMatch**
- Called when a rule's condition matches
- Receives the matched rule and its destinations
- Use for auditing or conditional actions

**OnEntityFetch**
- Called when entity data is fetched
- Receives entity data and any fetch error
- Use for caching, monitoring, or fallback handling

**OnRoutingError**
- Called when errors occur during routing
- Receives the error and message context
- Use for error tracking or alerting

### Callback Example: Status Updates

Update entity status based on routing results:

```go
callbacks := &routing.Callbacks{
    OnEntityFetch: func(ctx context.Context, entityID, entityType string, data map[string]any, err error) {
        if err != nil {
            // Update entity status to indicate fetch failure
            updateEntityStatus(ctx, entityID, "fetch_failed", err.Error())
        }
    },
    OnRuleMatch: func(ctx context.Context, msg *routing.Message, rule *routing.Rule, destinations []routing.Destination) {
        // Update entity with routing information
        if entityID, ok := msg.Event["entity_id"].(string); ok {
            updateEntityRouting(ctx, entityID, rule.Name, destinations)
        }
    },
    OnRoutingError: func(ctx context.Context, msg *routing.Message, err error) {
        // Update entity status on routing errors
        if entityID, ok := msg.Event["entity_id"].(string); ok {
            updateEntityStatus(ctx, entityID, "routing_error", err.Error())
        }
    },
}
```

### Callback Example: Metrics Collection

Collect routing metrics:

```go
var (
    routesEvaluated = prometheus.NewCounter(...)
    rulesMatched    = prometheus.NewCounterVec(...)
    entityFetches   = prometheus.NewHistogram(...)
)

callbacks := &routing.Callbacks{
    OnBeforeRoute: func(ctx context.Context, msg *routing.Message) error {
        routesEvaluated.Inc()
        return nil
    },
    OnRuleMatch: func(ctx context.Context, msg *routing.Message, rule *routing.Rule, destinations []routing.Destination) {
        rulesMatched.WithLabelValues(rule.Name).Inc()
    },
    OnEntityFetch: func(ctx context.Context, entityID, entityType string, data map[string]any, err error) {
        duration := time.Since(start).Seconds()
        entityFetches.Observe(duration)
    },
}
```

## CEL Documentation

For more information on CEL expressions, see:
- [CEL Language Definition](https://github.com/google/cel-spec)
- [CEL Go Implementation](https://github.com/google/cel-go)
