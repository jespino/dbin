package db

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"time"
)

func init() {
	Register(DatabaseInfo{
		Name:        "arango",
		Description: "ArangoDB multi-model database",
		Manager:     NewArangoManager,
	})
}

type ArangoManager struct {
	*BaseManager
}

func NewArangoManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &ArangoManager{
		BaseManager: base,
	}
}

func (am *ArangoManager) StartDatabase() error {
	ctx := context.Background()

	if err := am.PullImageIfNeeded(ctx, "arangodb:latest"); err != nil {
		return err
	}

	env := []string{
		"ARANGO_ROOT_PASSWORD=root",
		"ARANGO_NO_AUTH=1",
	}

	containerId, port, err := am.CreateContainer(ctx, "arangodb:latest", "dbin-arango", "8529/tcp", env, "/var/lib/arangodb3", nil)
	if err != nil {
		return err
	}
	am.dbContainerId = containerId
	am.dbPort = port

	log.Printf("ArangoDB is ready and listening on port %s\n", am.dbPort)
	return nil
}

func (am *ArangoManager) StartClient() error {
	return StartWebInterface(am.dbPort)
}

func (am *ArangoManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return am.BaseManager.Cleanup(ctx)
}
