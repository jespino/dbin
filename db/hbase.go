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
		Name:        "hbase",
		Description: "Apache HBase database",
		Manager:     NewHBaseManager,
	})
}

type HBaseManager struct {
	*BaseManager
	zookeeperContainerId string
	networkId            string
}

func NewHBaseManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &HBaseManager{
		BaseManager: base,
	}
}

func (hm *HBaseManager) StartDatabase() error {
	ctx := context.Background()

	// Pull required images
	images := []string{
		"zookeeper:latest",
		"harisekhon/hbase:latest",
	}
	for _, image := range images {
		if err := hm.PullImageIfNeeded(ctx, image); err != nil {
			return err
		}
	}

	// Create dedicated network for HBase containers
	networkName := "dbin-hbase-net"
	networkResponse, err := hm.dockerCli.NetworkCreate(ctx, networkName, network.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create network: %v", err)
	}
	hm.networkId = networkResponse.ID

	// Start ZooKeeper first
	zookeeperEnv := []string{}
	containerId, _, err := hm.CreateContainer(ctx, "zookeeper:latest", "dbin-zookeeper", "2181/tcp", zookeeperEnv, "/data", nil)
	if err != nil {
		return err
	}
	hm.zookeeperContainerId = containerId

	// Connect ZooKeeper to network
	if err := hm.dockerCli.NetworkConnect(ctx, hm.networkId, hm.zookeeperContainerId, nil); err != nil {
		return fmt.Errorf("failed to connect ZooKeeper to network: %v", err)
	}

	// Wait for ZooKeeper to be ready
	time.Sleep(10 * time.Second)

	// Start HBase
	hbaseEnv := []string{
		"HBASE_CONF_hbase_zookeeper_quorum=dbin-zookeeper",
	}

	containerId, port, err := hm.CreateContainer(ctx, "harisekhon/hbase:latest", "dbin-hbase", "16010/tcp", hbaseEnv, "/data", nil)
	if err != nil {
		return err
	}
	hm.dbContainerId = containerId
	hm.dbPort = port

	// Connect HBase to network
	if err := hm.dockerCli.NetworkConnect(ctx, hm.networkId, hm.dbContainerId, nil); err != nil {
		return fmt.Errorf("failed to connect HBase to network: %v", err)
	}

	log.Printf("HBase is ready! Web UI available on port %s\n", hm.dbPort)
	return nil
}

func (hm *HBaseManager) StartClient() error {
	return hm.StartContainerClient("hbase", "shell")
}

func (hm *HBaseManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Clean up containers
	containers := []string{hm.dbContainerId, hm.zookeeperContainerId}
	for _, id := range containers {
		if id != "" {
			if err := hm.dockerCli.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
				log.Printf("Warning: Failed to stop container %s: %v", id, err)
			}
			if err := hm.dockerCli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
				log.Printf("Warning: Failed to remove container %s: %v", id, err)
			}
		}
	}

	// Clean up network
	if hm.networkId != "" {
		if err := hm.dockerCli.NetworkRemove(ctx, hm.networkId); err != nil {
			log.Printf("Warning: Failed to remove network: %v", err)
		}
	}

	return nil
}
