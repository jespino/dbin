package db

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

// Common interface for database managers
type DatabaseManager interface {
	StartDatabase() error
	StartClient() error
	Cleanup() error
}

// Base structure for all database managers
type BaseManager struct {
	dataDir       string
	dockerCli     *client.Client
	dbContainerId string
	dbPort        string
}

// NewBaseManager creates a new base manager with Docker client
func NewBaseManager(dataDir string) (*BaseManager, error) {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithVersion("1.46"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %v", err)
	}

	return &BaseManager{
		dataDir:   dataDir,
		dockerCli: cli,
	}, nil
}

// PullImageIfNeeded pulls the Docker image if it's not present locally
func (bm *BaseManager) PullImageIfNeeded(ctx context.Context, imageName string) error {
	_, _, err := bm.dockerCli.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		log.Printf("%s image not found locally, pulling...\n", imageName)
		reader, err := bm.dockerCli.ImagePull(ctx, imageName, image.PullOptions{})
		if err != nil {
			return fmt.Errorf("failed to pull image: %v", err)
		}
		defer reader.Close()
		
		// Wait for the pull to complete
		_, err = io.Copy(os.Stdout, reader)
		if err != nil {
			return fmt.Errorf("error while pulling image: %v", err)
		}
	} else {
		log.Printf("Using existing %s image\n", imageName)
	}
	return nil
}

// CreateContainer creates a new container with the given configuration
func (bm *BaseManager) CreateContainer(
	ctx context.Context,
	imageName string,
	containerName string,
	port string,
	env []string,
	volumePath string,
) error {
	containerConfig := &container.Config{
		Image: imageName,
		Env:   env,
		ExposedPorts: nat.PortSet{
			nat.Port(port): struct{}{},
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			nat.Port(port): []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "0", // Let Docker assign a random port
				},
			},
		},
	}

	if bm.dataDir != "" && volumePath != "" {
		hostConfig.Binds = []string{
			fmt.Sprintf("%s:%s", bm.dataDir, volumePath),
		}
	}

	resp, err := bm.dockerCli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, containerName)
	if err != nil {
		return fmt.Errorf("failed to create container: %v", err)
	}

	bm.dbContainerId = resp.ID

	log.Println("Starting container...")
	if err := bm.dockerCli.ContainerStart(ctx, bm.dbContainerId, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %v", err)
	}
	log.Println("Container started successfully")

	// Get the assigned port
	inspect, err := bm.dockerCli.ContainerInspect(ctx, bm.dbContainerId)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %v", err)
	}

	bm.dbPort = inspect.NetworkSettings.Ports[nat.Port(port)][0].HostPort
	return nil
}

// Cleanup stops and removes the database container
func (bm *BaseManager) Cleanup(ctx context.Context) error {
	if bm.dbContainerId != "" {
		log.Printf("Stopping container %s...\n", bm.dbContainerId)
		if err := bm.dockerCli.ContainerStop(ctx, bm.dbContainerId, container.StopOptions{}); err != nil {
			return fmt.Errorf("failed to stop database container: %v", err)
		}
		log.Println("Container stopped successfully")

		log.Println("Removing container...")
		if err := bm.dockerCli.ContainerRemove(ctx, bm.dbContainerId, container.RemoveOptions{
			Force: true,
		}); err != nil {
			return fmt.Errorf("failed to remove database container: %v", err)
		}
		log.Println("Container removed successfully")
	}
	return nil
}
