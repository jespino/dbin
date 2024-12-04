package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	_ "github.com/lib/pq"
)

type PostgresManager struct {
	dataDir       string
	dockerCli     *client.Client
	dbContainerId string
	dbPort        string
}

func NewPostgresManager(dataDir string) *PostgresManager {
	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithVersion("1.46"),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to create Docker client: %v", err))
	}

	return &PostgresManager{
		dataDir:   dataDir,
		dockerCli: cli,
	}
}

func (pm *PostgresManager) StartDatabase() error {
	ctx := context.Background()

	_, _, err := pm.dockerCli.ImageInspectWithRaw(ctx, "postgres:latest")
	if err != nil {
		fmt.Println("PostgreSQL image not found locally, pulling...")
		reader, err := pm.dockerCli.ImagePull(ctx, "postgres:latest", image.PullOptions{})
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
		fmt.Println("Using existing PostgreSQL image")
	}

	fmt.Println("Creating container...")
	containerConfig := &container.Config{
		Image: "postgres:latest",
		Env: []string{
			"POSTGRES_PASSWORD=postgres",
			"POSTGRES_USER=postgres",
			"POSTGRES_DB=postgres",
		},
		ExposedPorts: nat.PortSet{
			"5432/tcp": struct{}{},
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"5432/tcp": []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: "0", // Let Docker assign a random port
				},
			},
		},
	}

	if pm.dataDir != "" {
		hostConfig.Binds = []string{
			fmt.Sprintf("%s:/var/lib/postgresql/data", pm.dataDir),
		}
	}

	resp, err := pm.dockerCli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "postgres-db")
	if err != nil {
		return fmt.Errorf("failed to create container: %v", err)
	}

	pm.dbContainerId = resp.ID

	fmt.Println("Starting container...")
	if err := pm.dockerCli.ContainerStart(ctx, pm.dbContainerId, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %v", err)
	}
	fmt.Println("Container started successfully")

	fmt.Println("Getting container port mapping...")
	inspect, err := pm.dockerCli.ContainerInspect(ctx, pm.dbContainerId)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %v", err)
	}

	pm.dbPort = inspect.NetworkSettings.Ports["5432/tcp"][0].HostPort

	fmt.Println("Waiting for database to be ready...")
	if err := pm.waitForDatabase(); err != nil {
		return fmt.Errorf("database failed to start: %v", err)
	}

	fmt.Printf("Database is ready and listening on port %s\n", pm.dbPort)
	return nil
}

func (pm *PostgresManager) waitForDatabase() error {
	connStr := fmt.Sprintf("host=localhost port=%s user=postgres password=postgres dbname=postgres sslmode=disable", pm.dbPort)

	for i := 0; i < 30; i++ {
		fmt.Printf("Attempting database connection (attempt %d/30)...\n", i+1)
		db, err := sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
			if err == nil {
				db.Close()
				return nil
			}
			db.Close()
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for database to be ready")
}

func (pm *PostgresManager) StartClient() error {
	cmd := exec.Command("docker", "exec", "-it", pm.dbContainerId, "psql", "-U", "postgres")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (pm *PostgresManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Stop and remove database container
	if pm.dbContainerId != "" {
		fmt.Printf("Stopping container %s...\n", pm.dbContainerId)
		if err := pm.dockerCli.ContainerStop(ctx, pm.dbContainerId, container.StopOptions{}); err != nil {
			return fmt.Errorf("failed to stop database container: %v", err)
		}
		fmt.Println("Container stopped successfully")

		fmt.Println("Removing container...")
		if err := pm.dockerCli.ContainerRemove(ctx, pm.dbContainerId, container.RemoveOptions{
			Force: true,
		}); err != nil {
			return fmt.Errorf("failed to remove database container: %v", err)
		}
		fmt.Println("Container removed successfully")
	}

	return nil
}
