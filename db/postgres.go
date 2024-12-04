package db

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"syscall"
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

	// Pull PostgreSQL image
	_, err := pm.dockerCli.ImagePull(ctx, "postgres:latest", image.PullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %v", err)
	}

	// Create container
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
		Binds: []string{
			fmt.Sprintf("%s:/var/lib/postgresql/data", pm.dataDir),
		},
	}

	resp, err := pm.dockerCli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "postgres-db")
	if err != nil {
		return fmt.Errorf("failed to create container: %v", err)
	}

	pm.dbContainerId = resp.ID

	// Start container
	if err := pm.dockerCli.ContainerStart(ctx, pm.dbContainerId, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %v", err)
	}

	// Get the actual bound port
	inspect, err := pm.dockerCli.ContainerInspect(ctx, pm.dbContainerId)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %v", err)
	}

	pm.dbPort = inspect.NetworkSettings.Ports["5432/tcp"][0].HostPort

	// Wait for database to be ready
	if err := pm.waitForDatabase(); err != nil {
		return fmt.Errorf("database failed to start: %v", err)
	}

	return nil
}

func (pm *PostgresManager) waitForDatabase() error {
	connStr := fmt.Sprintf("host=localhost port=%s user=postgres password=postgres dbname=postgres sslmode=disable", pm.dbPort)

	for i := 0; i < 30; i++ {
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
	args := []string{
		"docker", "exec",
		"-it",
		"--rm",
		pm.dbContainerId,
		"psql",
		"-U", "postgres",
	}

	// Replace current process with psql
	return syscall.Exec("/usr/bin/docker", args, os.Environ())
}

func (pm *PostgresManager) Cleanup() error {
	ctx := context.Background()

	// Stop and remove database container
	if pm.dbContainerId != "" {
		if err := pm.dockerCli.ContainerStop(ctx, pm.dbContainerId, container.StopOptions{}); err != nil {
			return fmt.Errorf("failed to stop database container: %v", err)
		}
		if err := pm.dockerCli.ContainerRemove(ctx, pm.dbContainerId, container.RemoveOptions{
			Force: true,
		}); err != nil {
			return fmt.Errorf("failed to remove database container: %v", err)
		}
	}

	return nil
}
