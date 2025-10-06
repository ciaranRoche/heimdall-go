package routing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfigFromYAML(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		wantErr  bool
		validate func(*testing.T, *Config)
	}{
		{
			name: "valid config with global settings",
			yaml: `
global:
  stop_on_first_match: true
  enable_metrics: true
  default_provider: "kafka"

entities:
  dinosaur:
    routing_rules:
      - name: "route_active"
        condition:
          expression: 'entity.status == "active"'
        publish:
          - topic: "dinosaur.active"
`,
			wantErr: false,
			validate: func(t *testing.T, c *Config) {
				if !c.StopOnFirstMatch {
					t.Error("Expected StopOnFirstMatch to be true")
				}
				if !c.EnableMetrics {
					t.Error("Expected EnableMetrics to be true")
				}
				if c.DefaultProvider != "kafka" {
					t.Errorf("Expected default provider kafka, got %s", c.DefaultProvider)
				}
				if len(c.Rules) != 1 {
					t.Errorf("Expected 1 rule, got %d", len(c.Rules))
				}
			},
		},
		{
			name: "entity with provider override",
			yaml: `
global:
  default_provider: "kafka"

entities:
  dinosaur:
    default_provider: "rabbitmq"
    routing_rules:
      - name: "route_active"
        condition:
          expression: 'entity.status == "active"'
        publish:
          - topic: "dinosaur.active"
`,
			wantErr: false,
			validate: func(t *testing.T, c *Config) {
				if len(c.Rules) != 1 {
					t.Fatalf("Expected 1 rule, got %d", len(c.Rules))
				}
				if c.Rules[0].Destinations[0].Provider != "rabbitmq" {
					t.Errorf("Expected provider rabbitmq, got %s", c.Rules[0].Destinations[0].Provider)
				}
			},
		},
		{
			name: "destination with explicit provider",
			yaml: `
global:
  default_provider: "kafka"

entities:
  dinosaur:
    default_provider: "rabbitmq"
    routing_rules:
      - name: "route_active"
        condition:
          expression: 'entity.status == "active"'
        publish:
          - topic: "dinosaur.active"
            provider: "nats"
`,
			wantErr: false,
			validate: func(t *testing.T, c *Config) {
				if len(c.Rules) != 1 {
					t.Fatalf("Expected 1 rule, got %d", len(c.Rules))
				}
				if c.Rules[0].Destinations[0].Provider != "nats" {
					t.Errorf("Expected provider nats, got %s", c.Rules[0].Destinations[0].Provider)
				}
			},
		},
		{
			name: "multiple entities with rules",
			yaml: `
global:
  default_provider: "kafka"

entities:
  dinosaur:
    routing_rules:
      - name: "route_active"
        condition:
          expression: 'entity.status == "active"'
        priority: 10
        publish:
          - topic: "dinosaur.active"
  habitat:
    routing_rules:
      - name: "route_available"
        condition:
          expression: 'entity.available == true'
        priority: 5
        publish:
          - topic: "habitat.available"
`,
			wantErr: false,
			validate: func(t *testing.T, c *Config) {
				if len(c.Rules) != 2 {
					t.Errorf("Expected 2 rules, got %d", len(c.Rules))
				}
				// Check priorities
				for _, rule := range c.Rules {
					if rule.Name == "route_active" && rule.Priority != 10 {
						t.Errorf("Expected priority 10, got %d", rule.Priority)
					}
					if rule.Name == "route_available" && rule.Priority != 5 {
						t.Errorf("Expected priority 5, got %d", rule.Priority)
					}
				}
			},
		},
		{
			name: "rule with headers and transform",
			yaml: `
global:
  default_provider: "kafka"

entities:
  dinosaur:
    routing_rules:
      - name: "route_with_headers"
        condition:
          expression: 'entity.status == "active"'
        publish:
          - topic: "dinosaur.active"
            headers:
              x-source: "heimdall"
              x-priority: "high"
            transform: "json_to_proto"
`,
			wantErr: false,
			validate: func(t *testing.T, c *Config) {
				if len(c.Rules) != 1 {
					t.Fatalf("Expected 1 rule, got %d", len(c.Rules))
				}
				dest := c.Rules[0].Destinations[0]
				if dest.Transform != "json_to_proto" {
					t.Errorf("Expected transform json_to_proto, got %s", dest.Transform)
				}
				if len(dest.Headers) != 2 {
					t.Errorf("Expected 2 headers, got %d", len(dest.Headers))
				}
				if dest.Headers["x-source"] != "heimdall" {
					t.Errorf("Expected header x-source=heimdall, got %v", dest.Headers["x-source"])
				}
			},
		},
		{
			name: "rule with stop_on_match",
			yaml: `
global:
  default_provider: "kafka"

entities:
  dinosaur:
    routing_rules:
      - name: "high_priority_exclusive"
        condition:
          expression: 'entity.critical == true'
        stop_on_match: true
        publish:
          - topic: "dinosaur.critical"
`,
			wantErr: false,
			validate: func(t *testing.T, c *Config) {
				if len(c.Rules) != 1 {
					t.Fatalf("Expected 1 rule, got %d", len(c.Rules))
				}
				if !c.Rules[0].StopOnMatch {
					t.Error("Expected StopOnMatch to be true")
				}
			},
		},
		{
			name: "invalid yaml syntax",
			yaml: `
global:
  default_provider: "kafka"
  invalid syntax here
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := LoadConfigFromYAML([]byte(tt.yaml))
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigFromYAML() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.validate != nil {
				tt.validate(t, config)
			}
		})
	}
}

func TestLoadConfigFromFile(t *testing.T) {
	// Create temp directory for test files
	tmpDir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		content  string
		wantErr  bool
	}{
		{
			name:     "valid file",
			filename: "valid.yaml",
			content: `
global:
  default_provider: "kafka"

entities:
  dinosaur:
    routing_rules:
      - name: "test"
        condition:
          expression: "true"
        publish:
          - topic: "test.topic"
`,
			wantErr: false,
		},
		{
			name:     "file not found",
			filename: "nonexistent.yaml",
			content:  "",
			wantErr:  true,
		},
		{
			name:     "invalid yaml in file",
			filename: "invalid.yaml",
			content:  "invalid: yaml: syntax: {",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filepath := filepath.Join(tmpDir, tt.filename)

			if tt.content != "" {
				err := os.WriteFile(filepath, []byte(tt.content), 0644)
				if err != nil {
					t.Fatalf("Failed to write test file: %v", err)
				}
			}

			_, err := LoadConfigFromFile(filepath)
			if (err != nil) != tt.wantErr {
				t.Errorf("LoadConfigFromFile() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestYAMLConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  YAMLConfig
		wantErr bool
	}{
		{
			name: "valid config",
			config: YAMLConfig{
				Global: GlobalSettings{
					DefaultProvider: "kafka",
				},
				Entities: map[string]EntityConfig{
					"dinosaur": {
						RoutingRules: []RuleConfig{
							{
								Name: "test",
								Condition: ConditionConfig{
									Expression: "true",
								},
								Publish: []DestinationConfig{
									{Topic: "test.topic"},
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "entity with no rules",
			config: YAMLConfig{
				Global: GlobalSettings{
					DefaultProvider: "kafka",
				},
				Entities: map[string]EntityConfig{
					"dinosaur": {
						RoutingRules: []RuleConfig{},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "rule with no name",
			config: YAMLConfig{
				Global: GlobalSettings{
					DefaultProvider: "kafka",
				},
				Entities: map[string]EntityConfig{
					"dinosaur": {
						RoutingRules: []RuleConfig{
							{
								Name: "",
								Condition: ConditionConfig{
									Expression: "true",
								},
								Publish: []DestinationConfig{
									{Topic: "test.topic"},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "rule with no condition",
			config: YAMLConfig{
				Global: GlobalSettings{
					DefaultProvider: "kafka",
				},
				Entities: map[string]EntityConfig{
					"dinosaur": {
						RoutingRules: []RuleConfig{
							{
								Name: "test",
								Condition: ConditionConfig{
									Expression: "",
								},
								Publish: []DestinationConfig{
									{Topic: "test.topic"},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "rule with no destinations",
			config: YAMLConfig{
				Global: GlobalSettings{
					DefaultProvider: "kafka",
				},
				Entities: map[string]EntityConfig{
					"dinosaur": {
						RoutingRules: []RuleConfig{
							{
								Name: "test",
								Condition: ConditionConfig{
									Expression: "true",
								},
								Publish: []DestinationConfig{},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "destination with no topic",
			config: YAMLConfig{
				Global: GlobalSettings{
					DefaultProvider: "kafka",
				},
				Entities: map[string]EntityConfig{
					"dinosaur": {
						RoutingRules: []RuleConfig{
							{
								Name: "test",
								Condition: ConditionConfig{
									Expression: "true",
								},
								Publish: []DestinationConfig{
									{Topic: ""},
								},
							},
						},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "no default provider but entity has explicit provider",
			config: YAMLConfig{
				Global: GlobalSettings{
					DefaultProvider: "",
				},
				Entities: map[string]EntityConfig{
					"dinosaur": {
						DefaultProvider: "kafka",
						RoutingRules: []RuleConfig{
							{
								Name: "test",
								Condition: ConditionConfig{
									Expression: "true",
								},
								Publish: []DestinationConfig{
									{Topic: "test.topic"},
								},
							},
						},
					},
				},
				Providers: map[string]ProviderConfig{
					"kafka": {Type: "kafka", Enabled: true},
				},
			},
			wantErr: false,
		},
		{
			name: "no default provider and no explicit providers",
			config: YAMLConfig{
				Global: GlobalSettings{
					DefaultProvider: "",
				},
				Entities: map[string]EntityConfig{
					"dinosaur": {
						RoutingRules: []RuleConfig{
							{
								Name: "test",
								Condition: ConditionConfig{
									Expression: "true",
								},
								Publish: []DestinationConfig{
									{Topic: "test.topic"},
								},
							},
						},
					},
				},
				Providers: map[string]ProviderConfig{
					"kafka": {Type: "kafka", Enabled: true},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProviderOverridePrecedence(t *testing.T) {
	yaml := `
global:
  default_provider: "kafka"

entities:
  dinosaur:
    default_provider: "rabbitmq"
    routing_rules:
      - name: "uses_entity_default"
        condition:
          expression: "true"
        publish:
          - topic: "topic1"
      - name: "uses_explicit_provider"
        condition:
          expression: "true"
        publish:
          - topic: "topic2"
            provider: "nats"
  habitat:
    routing_rules:
      - name: "uses_global_default"
        condition:
          expression: "true"
        publish:
          - topic: "topic3"
`

	config, err := LoadConfigFromYAML([]byte(yaml))
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Find each rule and check provider
	for _, rule := range config.Rules {
		switch rule.Name {
		case "uses_entity_default":
			if rule.Destinations[0].Provider != "rabbitmq" {
				t.Errorf("Expected rabbitmq, got %s", rule.Destinations[0].Provider)
			}
		case "uses_explicit_provider":
			if rule.Destinations[0].Provider != "nats" {
				t.Errorf("Expected nats, got %s", rule.Destinations[0].Provider)
			}
		case "uses_global_default":
			if rule.Destinations[0].Provider != "kafka" {
				t.Errorf("Expected kafka, got %s", rule.Destinations[0].Provider)
			}
		}
	}
}
