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
		Name:        "orientdb",
		Description: "OrientDB multi-model database",
		Manager:     NewOrientDBManager,
	})
}

type OrientDBManager struct {
	*BaseManager
}

func NewOrientDBManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &OrientDBManager{
		BaseManager: base,
	}
}

func (om *OrientDBManager) StartDatabase() error {
	ctx := context.Background()

	if err := om.PullImageIfNeeded(ctx, "orientdb:latest"); err != nil {
		return err
	}

	env := []string{
		"ORIENTDB_ROOT_PASSWORD=root",
	}

	containerId, port, err := om.CreateContainer(ctx, "orientdb:latest", "dbin-orientdb", "2480/tcp", env, "/orientdb/databases", nil)
	if err != nil {
		return err
	}
	om.dbContainerId = containerId
	om.dbPort = port

	log.Printf("OrientDB is ready and listening on port %s\n", om.dbPort)
	return nil
}

func (om *OrientDBManager) StartClient() error {
	return StartWebInterface(om.dbPort)
}

func (om *OrientDBManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return om.BaseManager.Cleanup(ctx)
}
