package db

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
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
	opensearchContainerId string
	dashboardsContainerId string
	dashboardsPort        string
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

	if err := om.PullImageIfNeeded(ctx, "opensearchproject/opensearch-dashboards:latest"); err != nil {
		return err
	}

	// Create a dedicated network for OpenSearch containers
	networkName := "dbin-opensearch-net"
	networkResponse, err := om.dockerCli.NetworkCreate(ctx, networkName, network.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create network: %v", err)
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
	om.opensearchContainerId = om.dbContainerId

	// Connect OpenSearch container to the network
	if err := om.dockerCli.NetworkConnect(ctx, networkResponse.ID, om.opensearchContainerId, nil); err != nil {
		return fmt.Errorf("failed to connect OpenSearch to network: %v", err)
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
	om.dashboardsContainerId = om.dbContainerId

	// Connect Dashboards container to the network
	if err := om.dockerCli.NetworkConnect(ctx, networkResponse.ID, om.dashboardsContainerId, nil); err != nil {
		return fmt.Errorf("failed to connect Dashboards to network: %v", err)
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

	// Clean up both containers and network
	if om.dashboardsContainerId != "" {
		if err := om.dockerCli.ContainerStop(ctx, om.dashboardsContainerId, container.StopOptions{}); err != nil {
			log.Printf("Warning: Failed to stop Dashboards container: %v", err)
		}
		if err := om.dockerCli.ContainerRemove(ctx, om.dashboardsContainerId, container.RemoveOptions{Force: true}); err != nil {
			log.Printf("Warning: Failed to remove Dashboards container: %v", err)
		}
	}

	if om.opensearchContainerId != "" {
		if err := om.dockerCli.ContainerStop(ctx, om.opensearchContainerId, container.StopOptions{}); err != nil {
			log.Printf("Warning: Failed to stop OpenSearch container: %v", err)
		}
		if err := om.dockerCli.ContainerRemove(ctx, om.opensearchContainerId, container.RemoveOptions{Force: true}); err != nil {
			log.Printf("Warning: Failed to remove OpenSearch container: %v", err)
		}
	}

	// Clean up the network
	networks, err := om.dockerCli.NetworkList(ctx, network.ListOptions{})
	if err == nil {
		for _, network := range networks {
			if network.Name == "dbin-opensearch-net" {
				if err := om.dockerCli.NetworkRemove(ctx, network.ID); err != nil {
					log.Printf("Warning: Failed to remove network: %v", err)
				}
				break
			}
		}
	}

	return nil
}
