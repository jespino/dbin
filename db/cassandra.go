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
		Name:        "cassandra",
		Description: "Cassandra database",
		Manager:     NewCassandraManager,
	})
}

type CassandraManager struct {
	*BaseManager
}

func NewCassandraManager(dataDir string) DatabaseManager {
	base, err := NewBaseManager(dataDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &CassandraManager{
		BaseManager: base,
	}
}

func (cm *CassandraManager) StartDatabase() error {
	ctx := context.Background()

	if err := cm.PullImageIfNeeded(ctx, "cassandra:latest"); err != nil {
		return err
	}

	if err := cm.CreateContainer(ctx, "cassandra:latest", "cassandra-db", "9042/tcp", nil, "/var/lib/cassandra"); err != nil {
		return err
	}

	// Wait for Cassandra to be ready
	log.Println("Waiting for Cassandra to be ready...")
	if err := cm.waitForDatabase(); err != nil {
		return fmt.Errorf("database failed to start: %v", err)
	}

	log.Printf("Cassandra is ready and listening on port %s\n", cm.dbPort)
	return nil
}

func (cm *CassandraManager) waitForDatabase() error {
	// Wait for Cassandra to be ready by checking nodetool status
	for i := 0; i < 30; i++ {
		log.Printf("Checking Cassandra status (attempt %d/30)...\n", i+1)
		cmd := exec.Command("docker", "exec", cm.dbContainerId, "nodetool", "status")
		if err := cmd.Run(); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timeout waiting for Cassandra to be ready")
}

func (cm *CassandraManager) StartClient() error {
	for i := 0; i < 5; i++ {
		cmd := exec.Command("docker", "exec", "-it", cm.dbContainerId, "cqlsh")
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

func (cm *CassandraManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return cm.BaseManager.Cleanup(ctx)
}
