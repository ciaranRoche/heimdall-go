package routing

import (
	"context"
	"fmt"
	"sync"
	"testing"
)

func TestCallbacks_OnBeforeRoute(t *testing.T) {
	var called bool
	var receivedMsg *Message

	callbacks := &Callbacks{
		OnBeforeRoute: func(ctx context.Context, msg *Message) error {
			called = true
			receivedMsg = msg
			return nil
		},
	}

	config := &Config{
		Callbacks: callbacks,
	}

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	err = engine.AddRule(&Rule{
		Name:      "test-rule",
		Condition: "true",
		Destinations: []Destination{
			{Topic: "test.topic"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	msg := &Message{
		Topic: "test",
		Entity: map[string]any{
			"status": "active",
		},
	}

	_, err = engine.Route(context.Background(), msg)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if !called {
		t.Error("OnBeforeRoute callback was not called")
	}

	if receivedMsg == nil {
		t.Fatal("OnBeforeRoute did not receive message")
	}

	if receivedMsg.Topic != "test" {
		t.Errorf("Expected topic 'test', got '%s'", receivedMsg.Topic)
	}
}

func TestCallbacks_OnBeforeRoute_Error(t *testing.T) {
	callbacks := &Callbacks{
		OnBeforeRoute: func(ctx context.Context, msg *Message) error {
			return fmt.Errorf("before route failed")
		},
	}

	config := &Config{
		Callbacks: callbacks,
	}

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	msg := &Message{
		Topic: "test",
	}

	_, err = engine.Route(context.Background(), msg)
	if err == nil {
		t.Error("Expected error from OnBeforeRoute callback")
	}
}

func TestCallbacks_OnAfterRoute(t *testing.T) {
	var called bool
	var receivedMsg *Message
	var receivedDest []Destination

	callbacks := &Callbacks{
		OnAfterRoute: func(ctx context.Context, msg *Message, destinations []Destination, err error) {
			called = true
			receivedMsg = msg
			receivedDest = destinations
		},
	}

	config := &Config{
		Callbacks: callbacks,
	}

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	err = engine.AddRule(&Rule{
		Name:      "test-rule",
		Condition: "true",
		Destinations: []Destination{
			{Topic: "test.topic"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

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

	if !called {
		t.Error("OnAfterRoute callback was not called")
	}

	if receivedMsg == nil {
		t.Fatal("OnAfterRoute did not receive message")
	}

	if len(receivedDest) != len(destinations) {
		t.Errorf("Expected %d destinations, got %d", len(destinations), len(receivedDest))
	}
}

func TestCallbacks_OnRuleMatch(t *testing.T) {
	var called bool
	var receivedRule *Rule
	var receivedDest []Destination

	callbacks := &Callbacks{
		OnRuleMatch: func(ctx context.Context, msg *Message, rule *Rule, destinations []Destination) {
			called = true
			receivedRule = rule
			receivedDest = destinations
		},
	}

	config := &Config{
		Callbacks: callbacks,
	}

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	testRule := &Rule{
		Name:      "match-rule",
		Condition: "entity.status == 'active'",
		Destinations: []Destination{
			{Topic: "matched.topic"},
		},
	}

	err = engine.AddRule(testRule)
	if err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	msg := &Message{
		Topic: "test",
		Entity: map[string]any{
			"status": "active",
		},
	}

	_, err = engine.Route(context.Background(), msg)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if !called {
		t.Error("OnRuleMatch callback was not called")
	}

	if receivedRule == nil {
		t.Fatal("OnRuleMatch did not receive rule")
	}

	if receivedRule.Name != "match-rule" {
		t.Errorf("Expected rule 'match-rule', got '%s'", receivedRule.Name)
	}

	if len(receivedDest) != 1 {
		t.Errorf("Expected 1 destination, got %d", len(receivedDest))
	}
}

func TestCallbacks_OnEntityFetch(t *testing.T) {
	var called bool
	var receivedEntityID, receivedEntityType string
	var receivedData map[string]any

	provider := &mockEntityProvider{
		entityType: "dinosaur",
		data: map[string]any{
			"species": "T-Rex",
			"status":  "active",
		},
	}

	callbacks := &Callbacks{
		OnEntityFetch: func(ctx context.Context, entityID, entityType string, data map[string]any, err error) {
			called = true
			receivedEntityID = entityID
			receivedEntityType = entityType
			receivedData = data
		},
	}

	config := &Config{
		EntityProviders: map[string]EntityDataProvider{
			"dinosaur": provider,
		},
		Callbacks: callbacks,
	}

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	err = engine.AddRule(&Rule{
		Name:      "test-rule",
		Condition: "true",
		Destinations: []Destination{
			{Topic: "test.topic"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	msg := &Message{
		Topic: "test",
		Event: map[string]any{
			"entity_id":   "123",
			"entity_type": "dinosaur",
		},
	}

	_, err = engine.Route(context.Background(), msg)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if !called {
		t.Error("OnEntityFetch callback was not called")
	}

	if receivedEntityID != "123" {
		t.Errorf("Expected entity ID '123', got '%s'", receivedEntityID)
	}

	if receivedEntityType != "dinosaur" {
		t.Errorf("Expected entity type 'dinosaur', got '%s'", receivedEntityType)
	}

	if receivedData == nil {
		t.Fatal("OnEntityFetch did not receive data")
	}

	if receivedData["species"] != "T-Rex" {
		t.Errorf("Expected species 'T-Rex', got '%v'", receivedData["species"])
	}
}

func TestCallbacks_OnRoutingError(t *testing.T) {
	var called bool
	var receivedErr error

	callbacks := &Callbacks{
		OnRoutingError: func(ctx context.Context, msg *Message, err error) {
			called = true
			receivedErr = err
		},
	}

	config := &Config{
		Callbacks: callbacks,
	}

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	// Add rule with invalid CEL expression that will cause evaluation error
	rule := &Rule{
		Name:      "bad-rule",
		Condition: "entity.nonexistent.deep.property",
		Destinations: []Destination{
			{Topic: "test.topic"},
		},
	}

	// Manually add rule to bypass validation
	ast, _ := engine.celEnv.Compile(rule.Condition)
	program, _ := engine.celEnv.Program(ast)
	rule.program = program
	engine.rules = append(engine.rules, rule)

	msg := &Message{
		Topic: "test",
		Entity: map[string]any{
			"status": "active",
		},
	}

	_, err = engine.Route(context.Background(), msg)
	if err != nil {
		t.Fatalf("Route() should not return error for rule evaluation errors: %v", err)
	}

	if !called {
		t.Error("OnRoutingError callback was not called")
	}

	if receivedErr == nil {
		t.Error("OnRoutingError did not receive error")
	}
}

func TestCallbacks_MultipleCallbacks(t *testing.T) {
	var beforeCalled, afterCalled, matchCalled bool
	var callOrder []string
	var mu sync.Mutex

	callbacks := &Callbacks{
		OnBeforeRoute: func(ctx context.Context, msg *Message) error {
			mu.Lock()
			beforeCalled = true
			callOrder = append(callOrder, "before")
			mu.Unlock()
			return nil
		},
		OnRuleMatch: func(ctx context.Context, msg *Message, rule *Rule, destinations []Destination) {
			mu.Lock()
			matchCalled = true
			callOrder = append(callOrder, "match")
			mu.Unlock()
		},
		OnAfterRoute: func(ctx context.Context, msg *Message, destinations []Destination, err error) {
			mu.Lock()
			afterCalled = true
			callOrder = append(callOrder, "after")
			mu.Unlock()
		},
	}

	config := &Config{
		Callbacks: callbacks,
	}

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	err = engine.AddRule(&Rule{
		Name:      "test-rule",
		Condition: "true",
		Destinations: []Destination{
			{Topic: "test.topic"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	msg := &Message{
		Topic: "test",
	}

	_, err = engine.Route(context.Background(), msg)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if !beforeCalled {
		t.Error("OnBeforeRoute was not called")
	}

	if !matchCalled {
		t.Error("OnRuleMatch was not called")
	}

	if !afterCalled {
		t.Error("OnAfterRoute was not called")
	}

	// Verify call order
	expectedOrder := []string{"before", "match", "after"}
	mu.Lock()
	for i, expected := range expectedOrder {
		if i >= len(callOrder) || callOrder[i] != expected {
			t.Errorf("Expected callback order %v, got %v", expectedOrder, callOrder)
			break
		}
	}
	mu.Unlock()
}

func TestCallbacks_NoCallbacks(t *testing.T) {
	// Test that routing works without callbacks
	config := &Config{
		Callbacks: nil, // No callbacks
	}

	engine, err := NewEngine(config)
	if err != nil {
		t.Fatalf("Failed to create engine: %v", err)
	}

	err = engine.AddRule(&Rule{
		Name:      "test-rule",
		Condition: "true",
		Destinations: []Destination{
			{Topic: "test.topic"},
		},
	})
	if err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	msg := &Message{
		Topic: "test",
	}

	destinations, err := engine.Route(context.Background(), msg)
	if err != nil {
		t.Fatalf("Route() error = %v", err)
	}

	if len(destinations) != 1 {
		t.Errorf("Expected 1 destination, got %d", len(destinations))
	}
}
