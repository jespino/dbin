package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

type MongoManager struct {
	*BaseManager
}

func NewMongoManager(dataDir string) *MongoManager {
	base, err := NewBaseManager(dataDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &MongoManager{
		BaseManager: base,
	}
}

func (mm *MongoManager) StartDatabase() error {
	ctx := context.Background()

	if err := mm.PullImageIfNeeded(ctx, "mongo:latest"); err != nil {
		return err
	}

	if err := mm.CreateContainer(ctx, "mongo:latest", "mongo-db", "27017/tcp", nil, "/data/db"); err != nil {
		return err
	}

	log.Printf("MongoDB is ready and listening on port %s\n", mm.dbPort)
	return nil
}

func (mm *MongoManager) StartClient() error {
	for i := 0; i < 3; i++ {
		cmd := exec.Command("docker", "exec", "-it", mm.dbContainerId, "mongosh")
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Run(); err == nil {
			return nil
		}
		
		if i < 2 { // Don't sleep after last attempt
			log.Printf("Failed to connect, retrying in 2 seconds (attempt %d/3)...", i+1)
			time.Sleep(2 * time.Second)
		}
	}
	return fmt.Errorf("failed to connect after 3 attempts")
}

func (mm *MongoManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return mm.BaseManager.Cleanup(ctx)
}
