// Package routing provides message routing capabilities with CEL expression support.
//
// The routing engine allows conditional message routing based on message content
// and entity data, supporting complex routing scenarios with fan-out capabilities.
package routing

import (
	"context"
	"fmt"

	"github.com/google/cel-go/cel"
)

// Router handles message routing based on configured rules.
type Router interface {
	// Route evaluates routing rules and returns destinations for the message
	Route(ctx context.Context, msg *Message) ([]Destination, error)

	// AddRule adds a routing rule to the router
	AddRule(rule *Rule) error

	// RemoveRule removes a routing rule by name
	RemoveRule(name string) error

	// GetRules returns all configured routing rules
	GetRules() []*Rule
}

// Message represents a message to be routed.
type Message struct {
	// Topic is the source topic/queue
	Topic string

	// Data is the message payload
	Data []byte

	// Headers contains message metadata
	Headers map[string]any

	// Entity contains entity data for routing decisions (optional)
	Entity map[string]any

	// Event contains event metadata
	Event map[string]any
}

// Destination represents a routing destination.
type Destination struct {
	// Topic is the destination topic/queue
	Topic string

	// Provider is the messaging provider to use (optional, uses default if empty)
	Provider string

	// Headers are additional headers to add to the message
	Headers map[string]any

	// Transform is an optional transformation to apply to the message
	Transform string
}

// Rule represents a routing rule with CEL condition.
type Rule struct {
	// Name is a unique identifier for the rule
	Name string

	// Condition is the CEL expression that must evaluate to true for routing
	Condition string

	// Destinations are the targets to publish to if condition matches
	Destinations []Destination

	// Priority determines rule evaluation order (higher = evaluated first)
	Priority int

	// StopOnMatch stops rule evaluation if this rule matches
	StopOnMatch bool

	// compiled CEL program (internal)
	program cel.Program
}

// EntityDataProvider provides entity data for routing decisions.
type EntityDataProvider interface {
	// GetEntity retrieves entity data by ID and type
	GetEntity(ctx context.Context, entityID, entityType string) (map[string]any, error)

	// EntityType returns the entity type this provider handles
	EntityType() string
}

// RouteResult contains the result of routing evaluation.
type RouteResult struct {
	// Matched indicates if any rules matched
	Matched bool

	// Destinations are the routing targets
	Destinations []Destination

	// MatchedRules lists which rules matched
	MatchedRules []string

	// EvaluationTime is how long routing took
	EvaluationTime int64
}

// Config contains routing engine configuration.
type Config struct {
	// Rules are the routing rules to evaluate
	Rules []*Rule

	// EntityProviders map entity types to data providers
	EntityProviders map[string]EntityDataProvider

	// DefaultProvider is the provider to use when destination doesn't specify one
	DefaultProvider string

	// StopOnFirstMatch stops evaluation after first matching rule
	StopOnFirstMatch bool

	// EnableMetrics enables routing metrics collection
	EnableMetrics bool

	// Callbacks for routing lifecycle events
	Callbacks *Callbacks
}

// Callbacks defines callback functions for routing lifecycle events.
type Callbacks struct {
	// OnBeforeRoute is called before routing evaluation starts
	OnBeforeRoute func(ctx context.Context, msg *Message) error

	// OnAfterRoute is called after routing evaluation completes
	OnAfterRoute func(ctx context.Context, msg *Message, destinations []Destination, err error)

	// OnRuleMatch is called when a rule matches
	OnRuleMatch func(ctx context.Context, msg *Message, rule *Rule, destinations []Destination)

	// OnEntityFetch is called when entity data is fetched
	OnEntityFetch func(ctx context.Context, entityID, entityType string, data map[string]any, err error)

	// OnRoutingError is called when an error occurs during routing
	OnRoutingError func(ctx context.Context, msg *Message, err error)
}

// Validate checks if the routing rule is valid.
func (r *Rule) Validate() error {
	if r.Name == "" {
		return fmt.Errorf("rule name is required")
	}

	if r.Condition == "" {
		return fmt.Errorf("rule condition is required")
	}

	if len(r.Destinations) == 0 {
		return fmt.Errorf("rule must have at least one destination")
	}

	for i, dest := range r.Destinations {
		if dest.Topic == "" {
			return fmt.Errorf("destination %d: topic is required", i)
		}
	}

	return nil
}

// Validate checks if the destination is valid.
func (d *Destination) Validate() error {
	if d.Topic == "" {
		return fmt.Errorf("destination topic is required")
	}
	return nil
}
