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
	dataDir          string
	port             int
	dockerCli        *client.Client
	dbContainerId    string
	clientContainerId string
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

	pm.dbContainerId = resp.ID

	// Start container
	if err := pm.dockerCli.ContainerStart(ctx, pm.dbContainerId, types.ContainerStartOptions{}); err != nil {
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

func (pm *PostgresManager) StartClient() error {
	ctx := context.Background()

	// Create client container
	containerConfig := &container.Config{
		Image: "postgres:latest",
		Cmd:   []string{"psql", "-h", "host.docker.internal", fmt.Sprintf("-p%d", pm.port), "-U", "postgres"},
		Tty:   true,
		AttachStdin: true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin: true,
		Env: []string{
			"PGPASSWORD=postgres",
		},
	}

	hostConfig := &container.HostConfig{
		NetworkMode: "host",
	}

	resp, err := pm.dockerCli.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return fmt.Errorf("failed to create client container: %v", err)
	}

	pm.clientContainerId = resp.ID

	// Attach to container
	attachResp, err := pm.dockerCli.ContainerAttach(ctx, pm.clientContainerId, types.ContainerAttachOptions{
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
	if err := pm.dockerCli.ContainerStart(ctx, pm.clientContainerId, types.ContainerStartOptions{}); err != nil {
		return fmt.Errorf("failed to start client container: %v", err)
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
	
	// Remove client container if it exists
	if pm.clientContainerId != "" {
		if err := pm.dockerCli.ContainerRemove(ctx, pm.clientContainerId, types.ContainerRemoveOptions{
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
		if err := pm.dockerCli.ContainerRemove(ctx, pm.dbContainerId, types.ContainerRemoveOptions{
			Force: true,
		}); err != nil {
			return fmt.Errorf("failed to remove database container: %v", err)
		}
	}

	return nil
}
