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
		Name:        "mongo",
		Description: "MongoDB database",
		Manager:     NewMongoManager,
	})
}

type MongoManager struct {
	*BaseManager
}

func NewMongoManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
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

	if err := mm.CreateContainer(ctx, "mongo:latest", "dbin-mongo", "27017/tcp", nil, "/data/db", nil); err != nil {
		return err
	}

	log.Printf("MongoDB is ready and listening on port %s\n", mm.dbPort)
	return nil
}

func (mm *MongoManager) StartClient() error {
	return mm.StartContainerClient("mongosh")
}

func (mm *MongoManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return mm.BaseManager.Cleanup(ctx)
}
