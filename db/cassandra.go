package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type CassandraManager struct {
	dataDir       string
	dockerCli     *client.Client
	dbContainerId string
	dbPort        string
}

func NewCassandraManager(dataDir string) *CassandraManager {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithVersion("1.46"),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to create Docker client: %v", err))
	}

	return &CassandraManager{
		dataDir:   dataDir,
		dockerCli: cli,
	}
}

func (cm *CassandraManager) StartDatabase() error {
	ctx := context.Background()

	_, _, err := cm.dockerCli.ImageInspectWithRaw(ctx, "cassandra:latest")
	if err != nil {
		log.Println("Cassandra image not found locally, pulling...")
		_, err := cm.dockerCli.ImagePull(ctx, "cassandra:latest", image.PullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull image: %v", err)
		}
		log.Println("Image pulled successfully")
	} else {
		log.Println("Using existing Cassandra image")
	}

	log.Println("Creating container...")
	containerConfig := &container.Config{
		Image: "cassandra:latest",
		ExposedPorts: nat.PortSet{
			"9042/tcp": struct{}{},
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"9042/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "0", // Let Docker assign a random port
				},
			},
		},
	}

	if cm.dataDir != "" {
		hostConfig.Binds = []string{
			fmt.Sprintf("%s:/var/lib/cassandra", cm.dataDir),
		}
	}

	resp, err := cm.dockerCli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "cassandra-db")
	if err != nil {
		return fmt.Errorf("failed to create container: %v", err)
	}

	cm.dbContainerId = resp.ID

	log.Println("Starting container...")
	if err := cm.dockerCli.ContainerStart(ctx, cm.dbContainerId, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %v", err)
	}
	log.Println("Container started successfully")

	log.Println("Getting container port mapping...")
	inspect, err := cm.dockerCli.ContainerInspect(ctx, cm.dbContainerId)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %v", err)
	}

	cm.dbPort = inspect.NetworkSettings.Ports["9042/tcp"][0].HostPort

	// Wait for Cassandra to be ready
	log.Println("Waiting for Cassandra to be ready...")
	if err := cm.waitForDatabase(); err != nil {
		return fmt.Errorf("database failed to start: %v", err)
	}

	log.Printf("Cassandra is ready and listening on port %s\n", cm.dbPort)
	return nil
}

func (cm *CassandraManager) waitForDatabase() error {
	// Wait for Cassandra to be ready by checking nodetool status
	for i := 0; i < 30; i++ {
		log.Printf("Checking Cassandra status (attempt %d/30)...\n", i+1)
		cmd := exec.Command("docker", "exec", cm.dbContainerId, "nodetool", "status")
		if err := cmd.Run(); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timeout waiting for Cassandra to be ready")
}

func (cm *CassandraManager) StartClient() error {
	cmd := exec.Command("docker", "exec", "-it", cm.dbContainerId, "cqlsh")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (cm *CassandraManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if cm.dbContainerId != "" {
		log.Printf("Stopping container %s...\n", cm.dbContainerId)
		if err := cm.dockerCli.ContainerStop(ctx, cm.dbContainerId, container.StopOptions{}); err != nil {
			return fmt.Errorf("failed to stop database container: %v", err)
		}
		log.Println("Container stopped successfully")

		log.Println("Removing container...")
		if err := cm.dockerCli.ContainerRemove(ctx, cm.dbContainerId, container.RemoveOptions{
			Force: true,
		}); err != nil {
			return fmt.Errorf("failed to remove database container: %v", err)
		}
		log.Println("Container removed successfully")
	}

	return nil
}
