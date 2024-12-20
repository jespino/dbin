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
		Name:        "surrealdb",
		Description: "SurrealDB database",
		Manager:     NewSurrealDBManager,
	})
}

type SurrealDBManager struct {
	*BaseManager
}

func NewSurrealDBManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &SurrealDBManager{
		BaseManager: base,
	}
}

func (sm *SurrealDBManager) StartDatabase() error {
	ctx := context.Background()

	if err := sm.PullImageIfNeeded(ctx, "surrealdb/surrealdb:latest"); err != nil {
		return err
	}

	env := []string{
		"SURREAL_USER=root",
		"SURREAL_PASS=root",
	}

	containerId, port, err := sm.CreateContainer(ctx, "surrealdb/surrealdb:latest", "dbin-surrealdb", "8000/tcp", env, "/data", []string{"start", "--user", "root", "--pass", "root"})
	if err != nil {
		return err
	}
	sm.dbContainerId = containerId
	sm.dbPort = port

	log.Printf("SurrealDB is ready and listening on port %s\n", sm.dbPort)
	return nil
}

func (sm *SurrealDBManager) StartClient() error {
	return sm.StartContainerClient("/surreal", "sql", "-u", "root", "-p", "root")
}

func (sm *SurrealDBManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return sm.BaseManager.Cleanup(ctx)
}
