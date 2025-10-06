package main

import (
	"context"
	"fmt"
	"log"

	"github.com/ciaranRoche/heimdall-go/routing"
)

func main() {
	// Load routing configuration from YAML file
	config, err := routing.LoadConfigFromFile("../config.yaml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Create routing engine with loaded config
	engine, err := routing.NewEngine(config)
	if err != nil {
		log.Fatalf("Failed to create engine: %v", err)
	}

	// Optional: Register entity data providers
	// engine.RegisterEntityProvider("dinosaur", &DinosaurProvider{})

	fmt.Println("Routing engine initialized successfully!")
	fmt.Printf("Loaded %d routing rules\n", len(config.Rules))
	fmt.Printf("Global settings:\n")
	fmt.Printf("  - Default Provider: %s\n", config.DefaultProvider)
	fmt.Printf("  - Stop on First Match: %v\n", config.StopOnFirstMatch)
	fmt.Printf("  - Metrics Enabled: %v\n", config.EnableMetrics)

	// Example: Route a dinosaur message
	msg := &routing.Message{
		Topic: "dinosaur.events",
		Entity: map[string]any{
			"species":      "T-Rex",
			"status":       "validated",
			"critical":     false,
			"threat_level": "medium",
		},
	}

	destinations, err := engine.Route(context.Background(), msg)
	if err != nil {
		log.Fatalf("Failed to route message: %v", err)
	}

	fmt.Printf("\nRouted to %d destinations:\n", len(destinations))
	for i, dest := range destinations {
		fmt.Printf("%d. Topic: %s, Provider: %s\n", i+1, dest.Topic, dest.Provider)
		if len(dest.Headers) > 0 {
			fmt.Printf("   Headers: %v\n", dest.Headers)
		}
	}

	// Example: Route a critical dinosaur
	criticalMsg := &routing.Message{
		Topic: "dinosaur.events",
		Entity: map[string]any{
			"species":      "Velociraptor",
			"status":       "active",
			"critical":     true,
			"threat_level": "high",
		},
	}

	criticalDests, err := engine.Route(context.Background(), criticalMsg)
	if err != nil {
		log.Fatalf("Failed to route critical message: %v", err)
	}

	fmt.Printf("\nCritical dinosaur routed to %d destinations:\n", len(criticalDests))
	for i, dest := range criticalDests {
		fmt.Printf("%d. Topic: %s, Provider: %s\n", i+1, dest.Topic, dest.Provider)
		if len(dest.Headers) > 0 {
			fmt.Printf("   Headers: %v\n", dest.Headers)
		}
	}

	// Example: Route a habitat message
	habitatMsg := &routing.Message{
		Topic: "habitat.events",
		Entity: map[string]any{
			"available": true,
			"capacity":  5,
			"status":    "active",
		},
	}

	habitatDests, err := engine.Route(context.Background(), habitatMsg)
	if err != nil {
		log.Fatalf("Failed to route habitat message: %v", err)
	}

	fmt.Printf("\nHabitat routed to %d destinations:\n", len(habitatDests))
	for i, dest := range habitatDests {
		fmt.Printf("%d. Topic: %s, Provider: %s\n", i+1, dest.Topic, dest.Provider)
	}

	// Display routing statistics
	stats := engine.Stats()
	fmt.Printf("\nRouting Statistics:\n")
	fmt.Printf("  Total Rules: %v\n", stats["total_rules"])
	fmt.Printf("  Entity Providers: %v\n", stats["entity_providers"])
	fmt.Printf("  Provider Types: %v\n", stats["provider_types"])
}
