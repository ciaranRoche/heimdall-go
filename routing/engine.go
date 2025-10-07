package routing

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/google/cel-go/cel"
)

// Engine implements the Router interface with CEL expression support.
type Engine struct {
	rules           []*Rule
	entityProviders map[string]EntityDataProvider
	config          *Config
	mu              sync.RWMutex
	celEnv          *cel.Env
}

// NewEngine creates a new routing engine with the given configuration.
func NewEngine(config *Config) (*Engine, error) {
	if config == nil {
		config = &Config{
			Rules:            []*Rule{},
			EntityProviders:  make(map[string]EntityDataProvider),
			StopOnFirstMatch: false,
		}
	}

	// Create CEL environment with standard declarations
	env, err := cel.NewEnv(
		cel.Variable("message", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("entity", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("event", cel.MapType(cel.StringType, cel.DynType)),
		cel.Variable("headers", cel.MapType(cel.StringType, cel.DynType)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	engine := &Engine{
		rules:           make([]*Rule, 0),
		entityProviders: config.EntityProviders,
		config:          config,
		celEnv:          env,
	}

	// Add and compile all rules
	for _, rule := range config.Rules {
		if err := engine.AddRule(rule); err != nil {
			return nil, fmt.Errorf("failed to add rule %s: %w", rule.Name, err)
		}
	}

	return engine, nil
}

// AddRule adds a routing rule to the engine.
func (e *Engine) AddRule(rule *Rule) error {
	if err := rule.Validate(); err != nil {
		return fmt.Errorf("invalid rule: %w", err)
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	// Check for duplicate rule names
	for _, existing := range e.rules {
		if existing.Name == rule.Name {
			return fmt.Errorf("rule with name %s already exists", rule.Name)
		}
	}

	// Compile CEL expression
	ast, issues := e.celEnv.Compile(rule.Condition)
	if issues != nil && issues.Err() != nil {
		return fmt.Errorf("failed to compile CEL expression: %w", issues.Err())
	}

	program, err := e.celEnv.Program(ast)
	if err != nil {
		return fmt.Errorf("failed to create CEL program: %w", err)
	}

	rule.program = program
	e.rules = append(e.rules, rule)

	// Sort rules by priority (higher priority first)
	sort.Slice(e.rules, func(i, j int) bool {
		return e.rules[i].Priority > e.rules[j].Priority
	})

	return nil
}

// RemoveRule removes a routing rule by name.
func (e *Engine) RemoveRule(name string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i, rule := range e.rules {
		if rule.Name == name {
			e.rules = append(e.rules[:i], e.rules[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("rule %s not found", name)
}

// GetRules returns all configured routing rules.
func (e *Engine) GetRules() []*Rule {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// Return a copy to prevent external modification
	rules := make([]*Rule, len(e.rules))
	copy(rules, e.rules)
	return rules
}

// Route evaluates routing rules and returns destinations for the message.
// nolint:gocyclo // Complexity is justified by comprehensive callback support and error handling
func (e *Engine) Route(ctx context.Context, msg *Message) ([]Destination, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	startTime := time.Now()

	// Call OnBeforeRoute callback
	if e.config.Callbacks != nil && e.config.Callbacks.OnBeforeRoute != nil {
		if err := e.config.Callbacks.OnBeforeRoute(ctx, msg); err != nil {
			if e.config.Callbacks.OnRoutingError != nil {
				e.config.Callbacks.OnRoutingError(ctx, msg, err)
			}
			return nil, fmt.Errorf("before route callback failed: %w", err)
		}
	}

	// Build CEL evaluation context
	evalCtx := map[string]any{
		"message": map[string]any{
			"topic": msg.Topic,
			"data":  msg.Data,
		},
		"headers": msg.Headers,
		"event":   msg.Event,
		"entity":  msg.Entity,
	}

	// If entity data is needed and not provided, try to fetch it
	if msg.Entity == nil && msg.Event != nil {
		if entityID, ok := msg.Event["entity_id"].(string); ok {
			if entityType, ok := msg.Event["entity_type"].(string); ok {
				if provider, exists := e.entityProviders[entityType]; exists {
					entityData, err := provider.GetEntity(ctx, entityID, entityType)

					// Call OnEntityFetch callback
					if e.config.Callbacks != nil && e.config.Callbacks.OnEntityFetch != nil {
						e.config.Callbacks.OnEntityFetch(ctx, entityID, entityType, entityData, err)
					}

					if err != nil {
						log.Printf("Routing: Failed to fetch entity data: %v", err)
					} else {
						evalCtx["entity"] = entityData
						msg.Entity = entityData
					}
				}
			}
		}
	}

	var allDestinations []Destination
	var matchedRules []string

	// Evaluate rules in priority order
	for _, rule := range e.rules {
		matched, err := e.evaluateRule(rule, evalCtx)
		if err != nil {
			log.Printf("Routing: Error evaluating rule %s: %v", rule.Name, err)
			if e.config.Callbacks != nil && e.config.Callbacks.OnRoutingError != nil {
				e.config.Callbacks.OnRoutingError(ctx, msg, err)
			}
			continue
		}

		if matched {
			matchedRules = append(matchedRules, rule.Name)
			allDestinations = append(allDestinations, rule.Destinations...)

			// Call OnRuleMatch callback
			if e.config.Callbacks != nil && e.config.Callbacks.OnRuleMatch != nil {
				e.config.Callbacks.OnRuleMatch(ctx, msg, rule, rule.Destinations)
			}

			if rule.StopOnMatch || e.config.StopOnFirstMatch {
				break
			}
		}
	}

	if e.config.EnableMetrics {
		elapsed := time.Since(startTime).Milliseconds()
		log.Printf("Routing: Evaluated %d rules in %dms, matched %d rules, %d destinations",
			len(e.rules), elapsed, len(matchedRules), len(allDestinations))
	}

	// Call OnAfterRoute callback
	if e.config.Callbacks != nil && e.config.Callbacks.OnAfterRoute != nil {
		e.config.Callbacks.OnAfterRoute(ctx, msg, allDestinations, nil)
	}

	return allDestinations, nil
}

// evaluateRule evaluates a single rule's condition.
func (e *Engine) evaluateRule(rule *Rule, ctx map[string]any) (bool, error) {
	if rule.program == nil {
		return false, fmt.Errorf("rule %s has no compiled program", rule.Name)
	}

	out, _, err := rule.program.Eval(ctx)
	if err != nil {
		return false, fmt.Errorf("CEL evaluation error: %w", err)
	}

	result, ok := out.Value().(bool)
	if !ok {
		return false, fmt.Errorf("CEL expression did not return boolean")
	}

	return result, nil
}

// RegisterEntityProvider registers an entity data provider.
func (e *Engine) RegisterEntityProvider(entityType string, provider EntityDataProvider) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.entityProviders == nil {
		e.entityProviders = make(map[string]EntityDataProvider)
	}

	e.entityProviders[entityType] = provider
	log.Printf("Routing: Registered entity provider for type: %s", entityType)
}

// GetEntityProvider retrieves an entity data provider by type.
func (e *Engine) GetEntityProvider(entityType string) (EntityDataProvider, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	provider, exists := e.entityProviders[entityType]
	return provider, exists
}

// Stats returns routing statistics.
func (e *Engine) Stats() map[string]any {
	e.mu.RLock()
	defer e.mu.RUnlock()

	return map[string]any{
		"total_rules":      len(e.rules),
		"entity_providers": len(e.entityProviders),
		"provider_types":   e.getProviderTypes(),
		"stop_on_first":    e.config.StopOnFirstMatch,
		"metrics_enabled":  e.config.EnableMetrics,
		"default_provider": e.config.DefaultProvider,
	}
}

func (e *Engine) getProviderTypes() []string {
	types := make([]string, 0, len(e.entityProviders))
	for typ := range e.entityProviders {
		types = append(types, typ)
	}
	return types
}
