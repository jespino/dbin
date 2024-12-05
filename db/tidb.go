package db

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
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
	pdContainerId   string
	tikvContainerId string
	networkId       string
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
	containerId, _, err := tm.CreateContainer(ctx, "pingcap/tidb:latest", "dbin-tidb", "4000/tcp", nil, "", []string{
		"--store=tikv",
		"--path=dbin-pd:2379",
	})
	if err != nil {
		return err
	}
	tm.dbContainerId = containerId

	// Connect TiDB to the network
	if err := tm.dockerCli.NetworkConnect(ctx, tm.networkId, tm.dbContainerId, nil); err != nil {
		return fmt.Errorf("failed to connect TiDB to network: %v", err)
	}

	return nil
}

func (tm *TiDBManager) StartClient() error {
	ctx := context.Background()

	// Pull MySQL client image if needed
	if err := tm.PullImageIfNeeded(ctx, "mysql:latest"); err != nil {
		return err
	}

	// Create MySQL client container
	clientConfig := &container.Config{
		Image:        "mysql:latest",
		Cmd:          []string{"mysql", "-hdbin-tidb", "-P4000", "-uroot", "--connect-timeout=10"},
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
	}

	hostConfig := &container.HostConfig{
		NetworkMode: container.NetworkMode("dbin-tidb-net"),
	}

	networkConfig := &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			"dbin-tidb-net": {},
		},
	}

	resp, err := tm.dockerCli.ContainerCreate(ctx, clientConfig, hostConfig, networkConfig, nil, "dbin-tidb-client")
	if err != nil {
		return fmt.Errorf("failed to create client container: %v", err)
	}

	// Wait for TiDB to be ready
	log.Println("Waiting for TiDB to be ready...")
	for i := 0; i < 30; i++ {
		log.Printf("Checking TiDB status (attempt %d/30)...\n", i+1)
		cmd := exec.Command("docker", "exec", resp.ID, "mysql", "-h127.0.0.1", "-P4000", "-uroot", "--connect-timeout=5", "-e", "SELECT 1")
		if err := cmd.Run(); err == nil {
			log.Printf("TiDB is ready and listening on port %s\n", tm.dbPort)
			return nil
		}
		if i < 29 {
			time.Sleep(2 * time.Second)
		} else {
			return fmt.Errorf("timeout waiting for TiDB to be ready")
		}
	}

	if err := tm.dockerCli.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start client container: %v", err)
	}

	// Attach to the container
	waiter, err := tm.dockerCli.ContainerAttach(ctx, resp.ID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("failed to attach to client container: %v", err)
	}
	defer waiter.Close()

	// Handle interactive session
	go io.Copy(os.Stdout, waiter.Reader)
	go io.Copy(os.Stderr, waiter.Reader)
	go io.Copy(waiter.Conn, os.Stdin)

	// Wait for container to exit
	statusCh, errCh := tm.dockerCli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for client container: %v", err)
		}
	case <-statusCh:
	}

	// Clean up client container
	if err := tm.dockerCli.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}); err != nil {
		log.Printf("Warning: Failed to remove client container: %v", err)
	}

	return nil
}

func (tm *TiDBManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Clean up client container if it exists
	containers, err := tm.dockerCli.ContainerList(ctx, container.ListOptions{All: true})
	if err == nil {
		for _, c := range containers {
			if c.Names[0] == "/dbin-tidb-client" {
				if err := tm.dockerCli.ContainerStop(ctx, c.ID, container.StopOptions{}); err != nil {
					log.Printf("Warning: Failed to stop client container: %v", err)
				}
				if err := tm.dockerCli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
					log.Printf("Warning: Failed to remove client container: %v", err)
				}
				break
			}
		}
	}

	// Clean up all service containers
	serviceContainers := map[string]string{
		tm.dbContainerId:   "TiDB",
		tm.tikvContainerId: "TiKV",
		tm.pdContainerId:   "PD",
	}

	// First disconnect all containers from the network
	if tm.networkId != "" {
		for id := range serviceContainers {
			if id != "" {
				if err := tm.dockerCli.NetworkDisconnect(ctx, tm.networkId, id, true); err != nil {
					log.Printf("Warning: Failed to disconnect container from network: %v", err)
				}
			}
		}
	}

	// Then stop and remove containers
	for id, name := range serviceContainers {
		if id != "" {
			if err := tm.dockerCli.ContainerStop(ctx, id, container.StopOptions{}); err != nil {
				log.Printf("Warning: Failed to stop %s container: %v", name, err)
			}
			if err := tm.dockerCli.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
				log.Printf("Warning: Failed to remove %s container: %v", name, err)
			}
		}
	}

	// Clean up the network last
	if tm.networkId != "" {
		// Give a small delay for network operations to complete
		time.Sleep(2 * time.Second)
		if err := tm.dockerCli.NetworkRemove(ctx, tm.networkId); err != nil {
			log.Printf("Warning: Failed to remove network: %v", err)
		}
	}

	return nil
}
