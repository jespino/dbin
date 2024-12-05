package db

import (
	_ "embed"
	"context"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
)

func init() {
	Register(DatabaseInfo{
		Name:        "tidb",
		Description: "TiDB distributed database",
		Manager:     NewTiDBManager,
	})
}

type TiDBManager struct {
	*BaseManager
	pdContainerId     string
	tikvContainerId   string
	networkId         string
}

func NewTiDBManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &TiDBManager{
		BaseManager: base,
	}
}

func (tm *TiDBManager) StartDatabase() error {
	ctx := context.Background()

	// Pull required images
	images := []string{
		"pingcap/pd:latest",
		"pingcap/tikv:latest",
		"pingcap/tidb:latest",
	}
	for _, image := range images {
		if err := tm.PullImageIfNeeded(ctx, image); err != nil {
			return err
		}
	}

	// Create dedicated network
	networkName := "dbin-tidb-net"
	networkResponse, err := tm.dockerCli.NetworkCreate(ctx, networkName, network.CreateOptions{})
	if err != nil {
		return fmt.Errorf("failed to create network: %v", err)
	}
	tm.networkId = networkResponse.ID

	// Start PD (Placement Driver)
	pdConfig := &container.Config{
		Image: "pingcap/pd:latest",
		Cmd: []string{
			"--name=pd1",
			"--data-dir=/data/pd",
			"--client-urls=http://0.0.0.0:2379",
			"--advertise-client-urls=http://dbin-pd:2379",
			"--peer-urls=http://0.0.0.0:2380",
			"--advertise-peer-urls=http://dbin-pd:2380",
			"--initial-cluster=pd1=http://dbin-pd:2380",
		},
	}

	pdHostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkName),
	}

	if tm.dataDir != "" {
		pdHostConfig.Binds = []string{
			fmt.Sprintf("%s/pd:/data/pd", tm.dataDir),
		}
	}

	pdResp, err := tm.dockerCli.ContainerCreate(ctx, pdConfig, pdHostConfig, nil, nil, "dbin-pd")
	if err != nil {
		return fmt.Errorf("failed to create PD container: %v", err)
	}
	tm.pdContainerId = pdResp.ID

	if err := tm.dockerCli.ContainerStart(ctx, tm.pdContainerId, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start PD container: %v", err)
	}

	// Wait for PD to be ready
	time.Sleep(5 * time.Second)

	// Start TiKV
	tikvConfig := &container.Config{
		Image: "pingcap/tikv:latest",
		Cmd: []string{
			"--pd=dbin-pd:2379",
			"--data-dir=/data/tikv",
			"--addr=0.0.0.0:20160",
			"--advertise-addr=dbin-tikv:20160",
		},
	}

	tikvHostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode(networkName),
	}

	if tm.dataDir != "" {
		tikvHostConfig.Binds = []string{
			fmt.Sprintf("%s/tikv:/data/tikv", tm.dataDir),
		}
	}

	tikvResp, err := tm.dockerCli.ContainerCreate(ctx, tikvConfig, tikvHostConfig, nil, nil, "dbin-tikv")
	if err != nil {
		return fmt.Errorf("failed to create TiKV container: %v", err)
	}
	tm.tikvContainerId = tikvResp.ID

	if err := tm.dockerCli.ContainerStart(ctx, tm.tikvContainerId, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start TiKV container: %v", err)
	}

	// Wait for TiKV to be ready
	time.Sleep(5 * time.Second)

	// Start TiDB
	if err := tm.CreateContainer(ctx, "pingcap/tidb:latest", "dbin-tidb", "4000/tcp", nil, "", []string{
		"--store=tikv",
		"--path=dbin-pd:2379",
	}); err != nil {
		return err
	}

	// Connect TiDB to the network
	if err := tm.dockerCli.NetworkConnect(ctx, tm.networkId, tm.dbContainerId, nil); err != nil {
		return fmt.Errorf("failed to connect TiDB to network: %v", err)
	}

	log.Printf("TiDB is ready and listening on port %s\n", tm.dbPort)
	return nil
}

func (tm *TiDBManager) StartClient() error {
	return tm.StartContainerClient("mysql", "-h127.0.0.1", "-P4000", "-uroot")
}

func (tm *TiDBManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Clean up all containers
	containers := []struct {
		id   string
		name string
	}{
		{tm.dbContainerId, "TiDB"},
		{tm.tikvContainerId, "TiKV"},
		{tm.pdContainerId, "PD"},
	}

	for _, c := range containers {
		if c.id != "" {
			if err := tm.dockerCli.ContainerStop(ctx, c.id, container.StopOptions{}); err != nil {
				log.Printf("Warning: Failed to stop %s container: %v", c.name, err)
			}
			if err := tm.dockerCli.ContainerRemove(ctx, c.id, container.RemoveOptions{Force: true}); err != nil {
				log.Printf("Warning: Failed to remove %s container: %v", c.name, err)
			}
		}
	}

	// Clean up the network
	if tm.networkId != "" {
		if err := tm.dockerCli.NetworkRemove(ctx, tm.networkId); err != nil {
			log.Printf("Warning: Failed to remove network: %v", err)
		}
	}

	return nil
}
