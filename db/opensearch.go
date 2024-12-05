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
		Name:        "opensearch",
		Description: "OpenSearch search engine",
		Manager:     NewOpenSearchManager,
	})
}

type OpenSearchManager struct {
	*BaseManager
}

func NewOpenSearchManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &OpenSearchManager{
		BaseManager: base,
	}
}

func (om *OpenSearchManager) StartDatabase() error {
	ctx := context.Background()

	if err := om.PullImageIfNeeded(ctx, "opensearchproject/opensearch:latest"); err != nil {
		return err
	}

	env := []string{
		"discovery.type=single-node",
		"OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m",
		"bootstrap.memory_lock=true",
		"DISABLE_SECURITY_PLUGIN=true",
	}

	if err := om.CreateContainer(ctx, "opensearchproject/opensearch:latest", "dbin-opensearch", "5601/tcp", env, "/usr/share/opensearch/data", nil); err != nil {
		return err
	}

	log.Printf("OpenSearch is ready and listening on port %s\n", om.dbPort)
	return nil
}

func (om *OpenSearchManager) StartClient() error {
	return StartWebInterface(om.dbPort)
}

func (om *OpenSearchManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return om.BaseManager.Cleanup(ctx)
}
