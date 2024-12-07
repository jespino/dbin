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
		Name:        "elasticsearch",
		Description: "Elasticsearch search engine",
		Manager:     NewElasticsearchManager,
	})
}

type ElasticsearchManager struct {
	*BaseManager
	elasticsearchContainerId string
	kibanaContainerId       string
	kibanaPort              string
}

func NewElasticsearchManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &ElasticsearchManager{
		BaseManager: base,
	}
}

func (em *ElasticsearchManager) StartDatabase() error {
	ctx := context.Background()

	if err := em.PullImageIfNeeded(ctx, "elasticsearch:8.12.0"); err != nil {
		return err
	}

	if err := em.PullImageIfNeeded(ctx, "kibana:8.12.0"); err != nil {
		return err
	}

	// Create a dedicated network for Elasticsearch containers
	networkName := "dbin-elasticsearch-net"
	networkResponse, err := em.dockerCli.NetworkCreate(ctx, networkName, network.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create network: %v", err)
	}

	// Start Elasticsearch container first
	env := []string{
		"discovery.type=single-node",
		"ES_JAVA_OPTS=-Xms512m -Xmx512m",
		"xpack.security.enabled=false",
		"bootstrap.memory_lock=true",
	}

	// Create Elasticsearch container with its native port
	containerId, port, err := em.CreateContainer(ctx, "elasticsearch:8.12.0", "dbin-elasticsearch", "9200/tcp", env, "/usr/share/elasticsearch/data", nil)
	if err != nil {
		return err
	}
	em.elasticsearchContainerId = containerId
	em.dbContainerId = containerId
	em.dbPort = port

	// Connect Elasticsearch container to the network
	if err := em.dockerCli.NetworkConnect(ctx, networkResponse.ID, em.elasticsearchContainerId, nil); err != nil {
		return fmt.Errorf("failed to connect Elasticsearch to network: %v", err)
	}

	// Wait for Elasticsearch to be ready
	time.Sleep(15 * time.Second)

	// Start Kibana container
	kibanaEnv := []string{
		"ELASTICSEARCH_HOSTS=http://dbin-elasticsearch:9200",
	}

	// Create Kibana container with port 5601
	containerId, port, err = em.CreateContainer(ctx, "kibana:8.12.0", "dbin-elasticsearch-kibana", "5601/tcp", kibanaEnv, "", nil)
	if err != nil {
		return err
	}
	em.kibanaPort = port
	em.kibanaContainerId = containerId

	// Connect Kibana container to the network
	if err := em.dockerCli.NetworkConnect(ctx, networkResponse.ID, em.kibanaContainerId, nil); err != nil {
		return fmt.Errorf("failed to connect Kibana to network: %v", err)
	}

	log.Printf("Elasticsearch is ready on port %s and Kibana is accessible on port %s\n", em.dbPort, em.kibanaPort)
	return nil
}

func (em *ElasticsearchManager) StartClient() error {
	return StartWebInterface(em.dbPort)
}

func (em *ElasticsearchManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Clean up both containers and network
	if em.kibanaContainerId != "" {
		if err := em.dockerCli.ContainerStop(ctx, em.kibanaContainerId, container.StopOptions{}); err != nil {
			log.Printf("Warning: Failed to stop Kibana container: %v", err)
		}
		if err := em.dockerCli.ContainerRemove(ctx, em.kibanaContainerId, container.RemoveOptions{Force: true}); err != nil {
			log.Printf("Warning: Failed to remove Kibana container: %v", err)
		}
	}

	if em.elasticsearchContainerId != "" {
		if err := em.dockerCli.ContainerStop(ctx, em.elasticsearchContainerId, container.StopOptions{}); err != nil {
			log.Printf("Warning: Failed to stop Elasticsearch container: %v", err)
		}
		if err := em.dockerCli.ContainerRemove(ctx, em.elasticsearchContainerId, container.RemoveOptions{Force: true}); err != nil {
			log.Printf("Warning: Failed to remove Elasticsearch container: %v", err)
		}
	}

	// Clean up the network
	networks, err := em.dockerCli.NetworkList(ctx, network.ListOptions{})
	if err == nil {
		for _, network := range networks {
			if network.Name == "dbin-elasticsearch-net" {
				if err := em.dockerCli.NetworkRemove(ctx, network.ID); err != nil {
					log.Printf("Warning: Failed to remove network: %v", err)
				}
				break
			}
		}
	}

	return nil
}
