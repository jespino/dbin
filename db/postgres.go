package db

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/go-connections/nat"
	_ "github.com/lib/pq"
)

type PostgresManager struct {
	dataDir           string
	dockerCli         *client.Client
	dbContainerId     string
	clientContainerId string
	dbPort            string
	networkName       string
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
		dataDir:     dataDir,
		dockerCli:   cli,
		networkName: "postgres-network",
	}
}

func (pm *PostgresManager) StartDatabase() error {
	ctx := context.Background()

	// Create network
	_, err := pm.dockerCli.NetworkCreate(ctx, pm.networkName, types.NetworkCreate{})
	if err != nil {
		return fmt.Errorf("failed to create network: %v", err)
	}

	// Pull PostgreSQL image
	_, err = pm.dockerCli.ImagePull(ctx, "postgres:latest", image.PullOptions{})
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

	// Connect container to network
	if err := pm.dockerCli.NetworkConnect(ctx, pm.networkName, pm.dbContainerId, nil); err != nil {
		return fmt.Errorf("failed to connect container to network: %v", err)
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

	// Finally remove the network
	networks, err := pm.dockerCli.NetworkList(ctx, types.NetworkListOptions{})
	if err == nil {
		for _, network := range networks {
			if network.Name == pm.networkName {
				if err := pm.dockerCli.NetworkRemove(ctx, network.ID); err != nil {
					fmt.Printf("Warning: failed to remove network: %v\n", err)
				}
				break
			}
		}
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
	ctx := context.Background()

	// Create client container
	containerConfig := &container.Config{
		Image:        "postgres:latest",
		Cmd:          []string{"psql", "-h", "postgres-db", "-p", "5432", "-U", "postgres"},
		Tty:          true,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		Env: []string{
			"PGPASSWORD=postgres",
		},
	}

	hostConfig := &container.HostConfig{}

	resp, err := pm.dockerCli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "postgres-client")
	if err != nil {
		return fmt.Errorf("failed to create client container: %v", err)
	}

	pm.clientContainerId = resp.ID

	// Attach to container
	attachResp, err := pm.dockerCli.ContainerAttach(ctx, pm.clientContainerId, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return fmt.Errorf("failed to attach to client container: %v", err)
	}
	defer attachResp.Close()

	// Start container
	if err := pm.dockerCli.ContainerStart(ctx, pm.clientContainerId, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start client container: %v", err)
	}

	// Connect container to network
	if err := pm.dockerCli.NetworkConnect(ctx, pm.networkName, pm.clientContainerId, nil); err != nil {
		return fmt.Errorf("failed to connect container to network: %v", err)
	}

	// Connect container IO with os.Stdin/os.Stdout
	go func() {
		_, err := attachResp.Conn.Write([]byte{})
		if err != nil {
			fmt.Printf("Error writing to container: %v\n", err)
		}
	}()

	go io.Copy(os.Stdout, attachResp.Reader)
	go io.Copy(attachResp.Conn, os.Stdin)

	// Wait for container to exit
	statusCh, errCh := pm.dockerCli.ContainerWait(ctx, pm.clientContainerId, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			return fmt.Errorf("error waiting for client container: %v", err)
		}
	case <-statusCh:
	}

	return nil
}

func (pm *PostgresManager) Cleanup() error {
	ctx := context.Background()

	// Disconnect containers from network first
	if pm.clientContainerId != "" {
		_ = pm.dockerCli.NetworkDisconnect(ctx, pm.networkName, pm.clientContainerId, true)
	}
	if pm.dbContainerId != "" {
		_ = pm.dockerCli.NetworkDisconnect(ctx, pm.networkName, pm.dbContainerId, true)
	}
	// Remove client container if it exists
	if pm.clientContainerId != "" {
		if err := pm.dockerCli.ContainerRemove(ctx, pm.clientContainerId, container.RemoveOptions{
			Force: true,
		}); err != nil {
			return fmt.Errorf("failed to remove client container: %v", err)
		}
	}

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
