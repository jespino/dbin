package db

import (
	_ "embed"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
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

func NewNeo4jManager(dataDir string) DatabaseManager {
	base, err := NewBaseManager(dataDir)
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

	if err := nm.CreateContainer(ctx, "neo4j:latest", "neo4j-db", "7687/tcp", env, "/data"); err != nil {
		return err
	}

	log.Printf("Neo4j is ready and listening on port %s\n", nm.dbPort)
	return nil
}

func (nm *Neo4jManager) StartClient() error {
	for i := 0; i < 5; i++ {
		cmd := exec.Command("docker", "exec", "-it", nm.dbContainerId, "cypher-shell", "-u", "neo4j", "-p", "password")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Run(); err == nil {
			return nil
		}
		
		if i < 4 { // Don't sleep after last attempt
			log.Printf("Failed to connect, retrying in 5 seconds (attempt %d/5)...", i+1)
			time.Sleep(5 * time.Second)
		}
	}
	return fmt.Errorf("failed to connect after 5 attempts")
}

func (nm *Neo4jManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return nm.BaseManager.Cleanup(ctx)
}
