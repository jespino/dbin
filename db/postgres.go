package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	_ "github.com/lib/pq"
)

type PostgresManager struct {
	dataDir    string
	port       int
	dockerCli  *client.Client
	containerId string
}

func NewPostgresManager(dataDir string, port int) *PostgresManager {
	cli, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		panic(fmt.Sprintf("Failed to create Docker client: %v", err))
	}

	return &PostgresManager{
		dataDir:   dataDir,
		port:      port,
		dockerCli: cli,
	}
}

func (pm *PostgresManager) StartDatabase() error {
	ctx := context.Background()

	// Pull PostgreSQL image
	_, err := pm.dockerCli.ImagePull(ctx, "postgres:latest", types.ImagePullOptions{})
	if err != nil {
		return fmt.Errorf("failed to pull image: %v", err)
	}

	// Create container
	portBinding := nat.PortBinding{
		HostIP:   "0.0.0.0",
		HostPort: fmt.Sprintf("%d", pm.port),
	}

	containerConfig := &container.Config{
		Image: "postgres:latest",
		Env: []string{
			"POSTGRES_PASSWORD=postgres",
			"POSTGRES_USER=postgres",
			"POSTGRES_DB=postgres",
		},
	}

	hostConfig := &container.HostConfig{
		PortBindings: nat.PortMap{
			"5432/tcp": []nat.PortBinding{portBinding},
		},
		Binds: []string{
			fmt.Sprintf("%s:/var/lib/postgresql/data", pm.dataDir),
		},
	}

	resp, err := pm.dockerCli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create container: %v", err)
	}

	pm.containerId = resp.ID

	// Start container
	if err := pm.dockerCli.ContainerStart(ctx, pm.containerId, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start container: %v", err)
	}

	// Wait for database to be ready
	if err := pm.waitForDatabase(); err != nil {
		return fmt.Errorf("database failed to start: %v", err)
	}

	return nil
}

func (pm *PostgresManager) waitForDatabase() error {
	connStr := fmt.Sprintf("host=localhost port=%d user=postgres password=postgres dbname=postgres sslmode=disable", pm.port)
	
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
