package routing

import (
	"context"
	"testing"
)

// mockEntityProvider implements EntityDataProvider for testing
type mockEntityProvider struct {
	entityType string
	data       map[string]any
}

func (m *mockEntityProvider) GetEntity(ctx context.Context, entityID, entityType string) (map[string]any, error) {
	return m.data, nil
}

func (m *mockEntityProvider) EntityType() string {
	return m.entityType
}

func TestNewEngine(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "nil config",
			config:  nil,
			wantErr: false,
		},
		{
			name: "valid config with rules",
			config: &Config{
				Rules: []*Rule{
					{
						Name:      "test-rule",
						Condition: "entity.status == 'active'",
						Destinations: []Destination{
							{Topic: "test.topic"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid CEL expression",
			config: &Config{
				Rules: []*Rule{
					{
						Name:      "bad-rule",
						Condition: "invalid syntax {{",
						Destinations: []Destination{
							{Topic: "test.topic"},
						},
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewEngine(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewEngine() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEngine_AddRule(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	tests := []struct {
		name    string
		rule    *Rule
		wantErr bool
	}{
		{
			name: "valid rule",
			rule: &Rule{
				Name:      "valid",
				Condition: "entity.status == 'active'",
				Destinations: []Destination{
					{Topic: "test.topic"},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate rule name",
			rule: &Rule{
				Name:      "valid",
				Condition: "entity.status == 'inactive'",
				Destinations: []Destination{
					{Topic: "test.topic2"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing name",
			rule: &Rule{
				Name:      "",
				Condition: "true",
				Destinations: []Destination{
					{Topic: "test.topic"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing condition",
			rule: &Rule{
				Name:      "no-condition",
				Condition: "",
				Destinations: []Destination{
					{Topic: "test.topic"},
				},
			},
			wantErr: true,
		},
		{
			name: "no destinations",
			rule: &Rule{
				Name:         "no-dest",
				Condition:    "true",
				Destinations: []Destination{},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := engine.AddRule(tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddRule() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEngine_Route(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Add test rules
	err = engine.AddRule(&Rule{
		Name:      "match-active",
		Condition: "entity.status == 'active'",
		Priority:  10,
		Destinations: []Destination{
			{Topic: "active.topic"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	err = engine.AddRule(&Rule{
		Name:      "match-type",
		Condition: "entity.type == 'dinosaur'",
		Priority:  5,
		Destinations: []Destination{
			{Topic: "dinosaur.topic"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	tests := []struct {
		name     string
		msg      *Message
		wantDest int
	}{
		{
			name: "matches first rule",
			msg: &Message{
				Topic: "test",
				Entity: map[string]any{
					"status": "active",
					"type":   "habitat",
				},
			},
			wantDest: 1,
		},
		{
			name: "matches both rules",
			msg: &Message{
				Topic: "test",
				Entity: map[string]any{
					"status": "active",
					"type":   "dinosaur",
				},
			},
			wantDest: 2,
		},
		{
			name: "matches no rules",
			msg: &Message{
				Topic: "test",
				Entity: map[string]any{
					"status": "inactive",
					"type":   "habitat",
				},
			},
			wantDest: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			destinations, err := engine.Route(context.Background(), tt.msg)
			if err != nil {
				t.Errorf("Route() error = %v", err)
				return
			}

			if len(destinations) != tt.wantDest {
				t.Errorf("Route() got %d destinations, want %d", len(destinations), tt.wantDest)
			}
		})
	}
}

func TestEngine_RouteWithStopOnMatch(t *testing.T) {
	config := &Config{
		StopOnFirstMatch: true,
	}

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Add multiple matching rules
	_ = engine.AddRule(&Rule{
		Name:      "rule1",
		Condition: "entity.status == 'active'",
		Priority:  10,
		Destinations: []Destination{
			{Topic: "topic1"},
		},
	})

	_ = engine.AddRule(&Rule{
		Name:      "rule2",
		Condition: "entity.status == 'active'",
		Priority:  5,
		Destinations: []Destination{
			{Topic: "topic2"},
		},
	})

	msg := &Message{
		Topic: "test",
		Entity: map[string]any{
			"status": "active",
		},
	}

	destinations, err := engine.Route(context.Background(), msg)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	// Should only match first rule due to StopOnFirstMatch
	if len(destinations) != 1 {
		t.Errorf("Expected 1 destination with StopOnFirstMatch, got %d", len(destinations))
	}

	if len(destinations) > 0 && destinations[0].Topic != "topic1" {
		t.Errorf("Expected topic1, got %s", destinations[0].Topic)
	}
}

func TestEngine_RegisterEntityProvider(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	provider := &mockEntityProvider{
		entityType: "dinosaur",
		data: map[string]any{
			"species": "T-Rex",
			"status":  "active",
		},
	}

	engine.RegisterEntityProvider("dinosaur", provider)

	retrievedProvider, exists := engine.GetEntityProvider("dinosaur")
	if !exists {
		t.Error("Expected provider to exist")
	}

	if retrievedProvider.EntityType() != "dinosaur" {
		t.Errorf("Expected entity type 'dinosaur', got '%s'", retrievedProvider.EntityType())
	}
}

func TestEngine_RemoveRule(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	rule := &Rule{
		Name:      "test-rule",
		Condition: "true",
		Destinations: []Destination{
			{Topic: "test.topic"},
		},
	}

	err = engine.AddRule(rule)
	if err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	// Remove existing rule
	err = engine.RemoveRule("test-rule")
	if err != nil {
		t.Errorf("RemoveRule() error = %v", err)
	}

	// Try to remove non-existent rule
	err = engine.RemoveRule("non-existent")
	if err == nil {
		t.Error("Expected error when removing non-existent rule")
	}
}

func TestEngine_GetRules(t *testing.T) {
	engine, err := NewEngine(nil)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	rules := []*Rule{
		{
			Name:      "rule1",
			Condition: "true",
			Priority:  10,
			Destinations: []Destination{
				{Topic: "topic1"},
			},
		},
		{
			Name:      "rule2",
			Condition: "true",
			Priority:  5,
			Destinations: []Destination{
				{Topic: "topic2"},
			},
		},
	}

	for _, rule := range rules {
		if err := engine.AddRule(rule); err != nil {
			t.Fatalf("Failed to add rule: %v", err)
		}
	}

	retrievedRules := engine.GetRules()
	if len(retrievedRules) != 2 {
		t.Errorf("Expected 2 rules, got %d", len(retrievedRules))
	}

	// Check rules are sorted by priority
	if retrievedRules[0].Priority < retrievedRules[1].Priority {
		t.Error("Rules should be sorted by priority (highest first)")
	}
}

func TestRule_Validate(t *testing.T) {
	tests := []struct {
		name    string
		rule    *Rule
		wantErr bool
	}{
		{
			name: "valid rule",
			rule: &Rule{
				Name:      "valid",
				Condition: "true",
				Destinations: []Destination{
					{Topic: "test.topic"},
				},
			},
			wantErr: false,
		},
		{
			name: "missing name",
			rule: &Rule{
				Name:      "",
				Condition: "true",
				Destinations: []Destination{
					{Topic: "test.topic"},
				},
			},
			wantErr: true,
		},
		{
			name: "missing condition",
			rule: &Rule{
				Name:      "test",
				Condition: "",
				Destinations: []Destination{
					{Topic: "test.topic"},
				},
			},
			wantErr: true,
		},
		{
			name: "no destinations",
			rule: &Rule{
				Name:         "test",
				Condition:    "true",
				Destinations: []Destination{},
			},
			wantErr: true,
		},
		{
			name: "destination missing topic",
			rule: &Rule{
				Name:      "test",
				Condition: "true",
				Destinations: []Destination{
					{Topic: ""},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.rule.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
