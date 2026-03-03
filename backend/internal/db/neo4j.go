package db

import (
	"context"
	"log"
	"os"

	"github.com/neo4j/neo4j-go-driver/v5/neo4j"
)

// NewNeo4jDriver creates a Neo4j driver using environment variables.
// Env vars: NEO4J_URI (default bolt://localhost:7687), NEO4J_USER (default neo4j), NEO4J_PASSWORD (default password).
func NewNeo4jDriver(ctx context.Context) (neo4j.DriverWithContext, error) {
	uri := os.Getenv("NEO4J_URI")
	if uri == "" {
		uri = "bolt://localhost:7687"
	}
	user := os.Getenv("NEO4J_USER")
	if user == "" {
		user = "neo4j"
	}
	password := os.Getenv("NEO4J_PASSWORD")
	if password == "" {
		password = "password"
	}

	driver, err := neo4j.NewDriverWithContext(uri, neo4j.BasicAuth(user, password, ""), func(config *neo4j.Config) {
		config.MaxConnectionPoolSize = 50
		config.MaxConnectionLifetime = 3 * 60 * 60 // 3 hours in seconds
	})
	if err != nil {
		return nil, err
	}

	if err := driver.VerifyConnectivity(ctx); err != nil {
		log.Printf("WARNING: neo4j connectivity check failed: %v", err)
		// Return the driver anyway — Neo4j may not be running yet (pipeline starts it).
	}

	log.Printf("Neo4j driver initialized: %s", uri)
	return driver, nil
}
