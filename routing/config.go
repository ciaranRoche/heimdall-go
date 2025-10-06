package routing

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// YAMLConfig represents the complete YAML configuration file structure.
type YAMLConfig struct {
	// Global settings
	Global GlobalSettings `yaml:"global"`

	// Entity-specific configurations
	Entities map[string]EntityConfig `yaml:"entities"`

	// Provider configurations (optional, for validation)
	Providers map[string]ProviderConfig `yaml:"providers,omitempty"`
}

// GlobalSettings contains global routing configuration.
type GlobalSettings struct {
	// StopOnFirstMatch stops evaluation after first matching rule
	StopOnFirstMatch bool `yaml:"stop_on_first_match"`

	// EnableMetrics enables routing metrics collection
	EnableMetrics bool `yaml:"enable_metrics"`

	// DefaultProvider is the provider to use when destination doesn't specify one
	DefaultProvider string `yaml:"default_provider"`
}

// EntityConfig contains routing configuration for a specific entity type.
type EntityConfig struct {
	// RoutingRules are the routing rules for this entity
	RoutingRules []RuleConfig `yaml:"routing_rules"`

	// DefaultProvider overrides global default for this entity
	DefaultProvider string `yaml:"default_provider,omitempty"`

	// StopOnFirstMatch overrides global setting for this entity
	StopOnFirstMatch *bool `yaml:"stop_on_first_match,omitempty"`
}

// RuleConfig represents a routing rule in YAML format.
type RuleConfig struct {
	// Name is a unique identifier for the rule
	Name string `yaml:"name"`

	// Condition is the CEL expression
	Condition ConditionConfig `yaml:"condition"`

	// Publish contains destination configurations
	Publish []DestinationConfig `yaml:"publish"`

	// Priority determines rule evaluation order (higher = evaluated first)
	Priority int `yaml:"priority,omitempty"`

	// StopOnMatch stops rule evaluation if this rule matches
	StopOnMatch bool `yaml:"stop_on_match,omitempty"`
}

// ConditionConfig represents a CEL condition.
type ConditionConfig struct {
	// Expression is the CEL expression string
	Expression string `yaml:"expression"`
}

// DestinationConfig represents a routing destination in YAML format.
type DestinationConfig struct {
	// Topic is the destination topic/queue
	Topic string `yaml:"topic"`

	// Provider is the messaging provider to use (optional)
	Provider string `yaml:"provider,omitempty"`

	// Headers are additional headers to add to the message
	Headers map[string]any `yaml:"headers,omitempty"`

	// Transform is an optional transformation to apply
	Transform string `yaml:"transform,omitempty"`
}

// ProviderConfig represents a provider configuration (for validation).
type ProviderConfig struct {
	// Type is the provider type (kafka, rabbitmq, etc.)
	Type string `yaml:"type"`

	// Enabled indicates if the provider is enabled
	Enabled bool `yaml:"enabled"`

	// Connection contains provider-specific connection details
	Connection map[string]any `yaml:"connection,omitempty"`
}

// LoadConfigFromFile loads routing configuration from a YAML file.
func LoadConfigFromFile(filepath string) (*Config, error) {
	// #nosec G304 - filepath is expected to be user-provided config path
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return LoadConfigFromYAML(data)
}

// LoadConfigFromYAML loads routing configuration from YAML bytes.
func LoadConfigFromYAML(data []byte) (*Config, error) {
	var yamlConfig YAMLConfig
	if err := yaml.Unmarshal(data, &yamlConfig); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	return yamlConfigToEngineConfig(&yamlConfig)
}

// yamlConfigToEngineConfig converts YAML config to engine Config.
func yamlConfigToEngineConfig(yc *YAMLConfig) (*Config, error) {
	config := &Config{
		Rules:            make([]*Rule, 0),
		EntityProviders:  make(map[string]EntityDataProvider),
		DefaultProvider:  yc.Global.DefaultProvider,
		StopOnFirstMatch: yc.Global.StopOnFirstMatch,
		EnableMetrics:    yc.Global.EnableMetrics,
	}

	// Convert entity-specific routing rules
	for entityType, entityConfig := range yc.Entities {
		for _, ruleConfig := range entityConfig.RoutingRules {
			rule, err := ruleConfigToRule(ruleConfig, entityType, &entityConfig, &yc.Global)
			if err != nil {
				return nil, fmt.Errorf("failed to convert rule %s for entity %s: %w",
					ruleConfig.Name, entityType, err)
			}
			config.Rules = append(config.Rules, rule)
		}
	}

	return config, nil
}

// ruleConfigToRule converts a YAML rule config to an engine Rule.
// nolint:unparam // error return kept for future validation logic
func ruleConfigToRule(rc RuleConfig, _ string, ec *EntityConfig, global *GlobalSettings) (*Rule, error) {
	rule := &Rule{
		Name:         rc.Name,
		Condition:    rc.Condition.Expression,
		Priority:     rc.Priority,
		StopOnMatch:  rc.StopOnMatch,
		Destinations: make([]Destination, len(rc.Publish)),
	}

	// Convert destinations
	for i, destConfig := range rc.Publish {
		dest := Destination{
			Topic:     destConfig.Topic,
			Provider:  destConfig.Provider,
			Headers:   destConfig.Headers,
			Transform: destConfig.Transform,
		}

		// Apply provider override logic
		if dest.Provider == "" {
			// Use entity-specific default if available
			if ec.DefaultProvider != "" {
				dest.Provider = ec.DefaultProvider
			} else {
				// Use global default
				dest.Provider = global.DefaultProvider
			}
		}

		rule.Destinations[i] = dest
	}

	return rule, nil
}

// Validate validates the YAML configuration.
func (yc *YAMLConfig) Validate() error {
	if err := yc.validateProviders(); err != nil {
		return err
	}

	return yc.validateEntities()
}

// validateProviders checks if provider configuration is valid.
func (yc *YAMLConfig) validateProviders() error {
	if yc.Global.DefaultProvider == "" && len(yc.Providers) > 0 {
		if !yc.hasExplicitProvider() {
			return fmt.Errorf("no default provider specified and no entity has explicit providers")
		}
	}
	return nil
}

// hasExplicitProvider checks if any entity or rule has an explicit provider.
func (yc *YAMLConfig) hasExplicitProvider() bool {
	for _, entity := range yc.Entities {
		if entity.DefaultProvider != "" {
			return true
		}
		for _, rule := range entity.RoutingRules {
			for _, dest := range rule.Publish {
				if dest.Provider != "" {
					return true
				}
			}
		}
	}
	return false
}

// validateEntities validates all entity configurations.
func (yc *YAMLConfig) validateEntities() error {
	for entityType, entityConfig := range yc.Entities {
		if err := validateEntityConfig(entityType, &entityConfig); err != nil {
			return err
		}
	}
	return nil
}

// validateEntityConfig validates a single entity configuration.
func validateEntityConfig(entityType string, ec *EntityConfig) error {
	if len(ec.RoutingRules) == 0 {
		return fmt.Errorf("entity %s has no routing rules", entityType)
	}

	for _, rule := range ec.RoutingRules {
		if err := validateRule(entityType, &rule); err != nil {
			return err
		}
	}
	return nil
}

// validateRule validates a single routing rule.
func validateRule(entityType string, rule *RuleConfig) error {
	if rule.Name == "" {
		return fmt.Errorf("entity %s has rule with empty name", entityType)
	}
	if rule.Condition.Expression == "" {
		return fmt.Errorf("entity %s rule %s has empty condition", entityType, rule.Name)
	}
	if len(rule.Publish) == 0 {
		return fmt.Errorf("entity %s rule %s has no publish destinations", entityType, rule.Name)
	}

	return validateDestinations(entityType, rule.Name, rule.Publish)
}

// validateDestinations validates destination configurations.
func validateDestinations(entityType, ruleName string, destinations []DestinationConfig) error {
	for i, dest := range destinations {
		if dest.Topic == "" {
			return fmt.Errorf("entity %s rule %s destination %d has empty topic",
				entityType, ruleName, i)
		}
	}
	return nil
}
