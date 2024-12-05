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
	dashboardsContainerId string
	dashboardsPort       string
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

	// Start OpenSearch container first
	env := []string{
		"discovery.type=single-node",
		"OPENSEARCH_JAVA_OPTS=-Xms512m -Xmx512m",
		"bootstrap.memory_lock=true",
		"DISABLE_SECURITY_PLUGIN=true",
		"OPENSEARCH_INITIAL_ADMIN_PASSWORD=admin",
	}

	// Create OpenSearch container with its native port
	if err := om.CreateContainer(ctx, "opensearchproject/opensearch:latest", "dbin-opensearch", "9200/tcp", env, "/usr/share/opensearch/data", nil); err != nil {
		return err
	}

	// Wait for OpenSearch to be ready
	time.Sleep(10 * time.Second)

	// Start OpenSearch Dashboards container
	dashboardsEnv := []string{
		"DISABLE_SECURITY_DASHBOARDS_PLUGIN=true",
		"OPENSEARCH_HOSTS=http://dbin-opensearch:9200",
	}

	// Create OpenSearch Dashboards container with port 5601
	if err := om.CreateContainer(ctx, "opensearchproject/opensearch-dashboards:latest", "dbin-opensearch-dashboards", "5601/tcp", dashboardsEnv, "", nil); err != nil {
		return err
	}

	log.Printf("OpenSearch is ready on port %s and Dashboards is accessible on port %s\n", om.dbPort, om.dashboardsPort)
	return nil
}

func (om *OpenSearchManager) StartClient() error {
	return StartWebInterface(om.dbPort)
}

func (om *OpenSearchManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Clean up both containers
	if om.dashboardsContainerId != "" {
		if err := om.dockerCli.ContainerStop(ctx, om.dashboardsContainerId, container.StopOptions{}); err != nil {
			log.Printf("Warning: Failed to stop Dashboards container: %v", err)
		}
		if err := om.dockerCli.ContainerRemove(ctx, om.dashboardsContainerId, container.RemoveOptions{Force: true}); err != nil {
			log.Printf("Warning: Failed to remove Dashboards container: %v", err)
		}
	}

	return om.BaseManager.Cleanup(ctx)
}
