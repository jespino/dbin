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
		Name:        "dgraph",
		Description: "Dgraph graph database",
		Manager:     NewDgraphManager,
	})
}

type DgraphManager struct {
	*BaseManager
	zeroContainerId  string
	alphaContainerId string
	ratelPort        string
}

func NewDgraphManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &DgraphManager{
		BaseManager: base,
	}
}

func (dm *DgraphManager) StartDatabase() error {
	ctx := context.Background()

	if err := dm.PullImageIfNeeded(ctx, "dgraph/dgraph:latest"); err != nil {
		return err
	}

	// Create a dedicated network for Dgraph containers
	networkName := "dbin-dgraph-net"
	networkResponse, err := dm.dockerCli.NetworkCreate(ctx, networkName, network.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create network: %v", err)
	}

	// Start Dgraph Zero
	zeroEnv := []string{}
	zeroCmd := []string{"dgraph", "zero", "--my=zero:5080"}

	containerId, _, err := dm.CreateContainer(ctx, "dgraph/dgraph:latest", "dbin-dgraph-zero", "5080/tcp", zeroEnv, "/dgraph", zeroCmd)
	if err != nil {
		return err
	}
	dm.zeroContainerId = containerId

	// Connect Zero container to the network
	if err := dm.dockerCli.NetworkConnect(ctx, networkResponse.ID, dm.zeroContainerId, nil); err != nil {
		return fmt.Errorf("failed to connect Zero to network: %v", err)
	}

	// Wait for Zero to be ready
	time.Sleep(5 * time.Second)

	// Start Dgraph Alpha
	alphaEnv := []string{}
	alphaCmd := []string{"dgraph", "alpha", "--my=alpha:7080", "--zero=zero:5080", "--security=whitelist", "--whitelist=0.0.0.0/0"}

	containerId, port, err := dm.CreateContainer(ctx, "dgraph/dgraph:latest", "dbin-dgraph-alpha", "8080/tcp", alphaEnv, "/dgraph", alphaCmd)
	if err != nil {
		return err
	}
	dm.alphaContainerId = containerId
	dm.dbContainerId = containerId // Set this for base manager compatibility
	dm.dbPort = port

	// Connect Alpha container to the network
	if err := dm.dockerCli.NetworkConnect(ctx, networkResponse.ID, dm.alphaContainerId, nil); err != nil {
		return fmt.Errorf("failed to connect Alpha to network: %v", err)
	}

	// Pull and start Ratel UI
	if err := dm.PullImageIfNeeded(ctx, "dgraph/ratel:latest"); err != nil {
		return err
	}

	ratelEnv := []string{
		"DGRAPH_ENDPOINT=	dbin-dgraph-alpha:8080",
	}
	ratelCmd := []string{"/usr/local/bin/dgraph-ratel"} // Correct path to executable

	containerId, port, err = dm.CreateContainer(ctx, "dgraph/ratel:latest", "dbin-dgraph-ratel", "8000/tcp", ratelEnv, "", ratelCmd)
	if err != nil {
		return err
	}
	dm.ratelPort = port

	log.Printf("Dgraph is ready! GraphQL endpoint on port %s, Ratel UI on port %s\n", dm.dbPort, dm.ratelPort)
	return nil
}

func (dm *DgraphManager) StartClient() error {
	return StartWebInterface(dm.ratelPort)
}

func (dm *DgraphManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Clean up all containers
	containers := []string{dm.alphaContainerId, dm.zeroContainerId}
	for _, id := range containers {
		if id != "" {
			if err := dm.dockerCli.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
				log.Printf("Warning: Failed to stop container %s: %v", id, err)
			}
			if err := dm.dockerCli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
				log.Printf("Warning: Failed to remove container %s: %v", id, err)
			}
		}
	}

	// Clean up the network
	networks, err := dm.dockerCli.NetworkList(ctx, network.ListOptions{})
	if err == nil {
		for _, network := range networks {
			if network.Name == "dbin-dgraph-net" {
				if err := dm.dockerCli.NetworkRemove(ctx, network.ID); err != nil {
					log.Printf("Warning: Failed to remove network: %v", err)
				}
				break
			}
		}
	}

	return nil
}
