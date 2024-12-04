package db

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
)

type MongoManager struct {
	dataDir       string
	dockerCli     *client.Client
	dbContainerId string
	dbPort        string
}

func NewMongoManager(dataDir string) *MongoManager {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithVersion("1.46"),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to create Docker client: %v", err))
	}

	return &MongoManager{
		dataDir:   dataDir,
		dockerCli: cli,
	}
}

func (mm *MongoManager) StartDatabase() error {
	ctx := context.Background()

	_, _, err := mm.dockerCli.ImageInspectWithRaw(ctx, "mongo:latest")
	if err != nil {
		log.Println("MongoDB image not found locally, pulling...")
		reader, err := mm.dockerCli.ImagePull(ctx, "mongo:latest", image.PullOptions{})
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
		log.Println("Using existing MongoDB image")
	}

	log.Println("Creating container...")
	containerConfig := &container.Config{
		Image: "mongo:latest",
		ExposedPorts: nat.PortSet{
			"27017/tcp": struct{}{},
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"27017/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "0", // Let Docker assign a random port
				},
			},
		},
	}

	if mm.dataDir != "" {
		hostConfig.Binds = []string{
			fmt.Sprintf("%s:/data/db", mm.dataDir),
		}
	}

	resp, err := mm.dockerCli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "mongo-db")
	if err != nil {
		return fmt.Errorf("failed to create container: %v", err)
	}

	mm.dbContainerId = resp.ID

	log.Println("Starting container...")
	if err := mm.dockerCli.ContainerStart(ctx, mm.dbContainerId, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %v", err)
	}
	log.Println("Container started successfully")

	log.Println("Getting container port mapping...")
	inspect, err := mm.dockerCli.ContainerInspect(ctx, mm.dbContainerId)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %v", err)
	}

	mm.dbPort = inspect.NetworkSettings.Ports["27017/tcp"][0].HostPort

	log.Printf("MongoDB is ready and listening on port %s\n", mm.dbPort)
	return nil
}

func (mm *MongoManager) StartClient() error {
	cmd := exec.Command("docker", "exec", "-it", mm.dbContainerId, "mongosh")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (mm *MongoManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if mm.dbContainerId != "" {
		log.Printf("Stopping container %s...\n", mm.dbContainerId)
		if err := mm.dockerCli.ContainerStop(ctx, mm.dbContainerId, container.StopOptions{}); err != nil {
			return fmt.Errorf("failed to stop database container: %v", err)
		}
		log.Println("Container stopped successfully")

		log.Println("Removing container...")
		if err := mm.dockerCli.ContainerRemove(ctx, mm.dbContainerId, container.RemoveOptions{
			Force: true,
		}); err != nil {
			return fmt.Errorf("failed to remove database container: %v", err)
		}
		log.Println("Container removed successfully")
	}

	return nil
}
