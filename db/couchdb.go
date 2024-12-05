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
		Name:        "couchdb",
		Description: "CouchDB database",
		Manager:     NewCouchDBManager,
	})
}

type CouchDBManager struct {
	*BaseManager
}

func NewCouchDBManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &CouchDBManager{
		BaseManager: base,
	}
}

func (cm *CouchDBManager) StartDatabase() error {
	ctx := context.Background()

	if err := cm.PullImageIfNeeded(ctx, "couchdb:latest"); err != nil {
		return err
	}

	env := []string{
		"COUCHDB_USER=admin",
		"COUCHDB_PASSWORD=password",
	}

	if err := cm.CreateContainer(ctx, "couchdb:latest", "dbin-couchdb", "5984/tcp", env, "/opt/couchdb/data", nil); err != nil {
		return err
	}

	log.Printf("CouchDB is ready and listening on port %s\n", cm.dbPort)
	return nil
}

func (cm *CouchDBManager) StartClient() error {
	return StartWebInterface(cm.dbPort + "/_utils")
}

func (cm *CouchDBManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return cm.BaseManager.Cleanup(ctx)
}
