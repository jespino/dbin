package db

import (
	_ "embed"
	"context"
	"fmt"
	"log"
	"time"
)

func init() {
	Register(DatabaseInfo{
		Name:        "neo4j",
		Description: "Neo4j database",
		Manager:     NewNeo4jManager,
	})
}

type Neo4jManager struct {
	*BaseManager
}

func NewNeo4jManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &Neo4jManager{
		BaseManager: base,
	}
}

func (nm *Neo4jManager) StartDatabase() error {
	ctx := context.Background()

	if err := nm.PullImageIfNeeded(ctx, "neo4j:latest"); err != nil {
		return err
	}

	env := []string{
		"NEO4J_AUTH=neo4j/password",
	}

	if err := nm.CreateContainer(ctx, "neo4j:latest", "dbin-neo4j", "7687/tcp", env, "/data", nil); err != nil {
		return err
	}

	log.Printf("Neo4j is ready and listening on port %s\n", nm.dbPort)
	return nil
}

func (nm *Neo4jManager) StartClient() error {
	return nm.StartContainerClient("cypher-shell", "-u", "neo4j", "-p", "password")
}

func (nm *Neo4jManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return nm.BaseManager.Cleanup(ctx)
}
