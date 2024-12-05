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
		Name:        "rethinkdb",
		Description: "RethinkDB database",
		Manager:     NewRethinkDBManager,
	})
}

type RethinkDBManager struct {
	*BaseManager
}

func NewRethinkDBManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &RethinkDBManager{
		BaseManager: base,
	}
}

func (rm *RethinkDBManager) StartDatabase() error {
	ctx := context.Background()

	if err := rm.PullImageIfNeeded(ctx, "rethinkdb:latest"); err != nil {
		return err
	}

	if err := rm.CreateContainer(ctx, "rethinkdb:latest", "rethinkdb-db", "8080/tcp", nil, "/data", nil); err != nil {
		return err
	}

	log.Printf("RethinkDB is ready and listening on port %s\n", rm.dbPort)
	return nil
}

func (rm *RethinkDBManager) StartClient() error {
	return StartWebInterface(rm.dbPort)
}

func (rm *RethinkDBManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return rm.BaseManager.Cleanup(ctx)
}
