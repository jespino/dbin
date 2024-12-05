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
		Name:        "mongo",
		Description: "MongoDB database",
		Manager:     NewMongoManager,
	})
}

type MongoManager struct {
	*BaseManager
}

func NewMongoManager(dataDir string) DatabaseManager {
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
	for i := 0; i < 5; i++ {
		cmd := exec.Command("docker", "exec", "-it", mm.dbContainerId, "mongosh")
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

func (mm *MongoManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return mm.BaseManager.Cleanup(ctx)
}
