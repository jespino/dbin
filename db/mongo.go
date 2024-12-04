package db

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
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
	cmd := exec.Command("docker", "exec", "-it", mm.dbContainerId, "mongosh")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (mm *MongoManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return mm.BaseManager.Cleanup(ctx)
}
