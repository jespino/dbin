package db

import (
	"context"
	"encoding/json"
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

type dockerMessage struct {
	Status         string `json:"status"`
	ID            string `json:"id"`
	Progress      string `json:"progress"`
	ProgressDetail struct {
		Current int64 `json:"current"`
		Total   int64 `json:"total"`
	} `json:"progressDetail"`
}

type progressInfo struct {
	current int64
	total   int64
}

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
	debug         bool
}

// NewBaseManager creates a new base manager with Docker client
func NewBaseManager(dataDir string, debug bool) (*BaseManager, error) {
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
		debug:     debug,
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
		
		// Process and display pull progress
		decoder := json.NewDecoder(reader)
		type layerProgress struct {
			id   string
			info *progressInfo
		}
		var layers []layerProgress
		layerMap := make(map[string]int) // Map ID to index in layers slice

		for decoder.More() {
			var msg dockerMessage
			if err := decoder.Decode(&msg); err != nil {
				if err == io.EOF {
					break
				}
				return fmt.Errorf("error decoding docker message: %v", err)
			}

			if msg.Status == "Downloading" && msg.Progress != "" {
				idx, exists := layerMap[msg.ID]
				if !exists {
					// New layer, add to slice
					layers = append(layers, layerProgress{
						id:   msg.ID,
						info: &progressInfo{},
					})
					idx = len(layers) - 1
					layerMap[msg.ID] = idx
				}

				// Update progress
				current := msg.ProgressDetail.Current
				total := msg.ProgressDetail.Total
				layers[idx].info.current = current
				layers[idx].info.total = total

				// Clear previous lines
				for range layers {
					fmt.Print("\033[1A\033[K") // Move up and clear line
				}

				// Print progress for all layers in order
				for _, layer := range layers {
					if layer.info.total > 0 {
						percentage := float64(layer.info.current) / float64(layer.info.total) * 100
						fmt.Printf("Downloading %s: %.1f%% of %.2f MB\n",
							layer.id[:12],
							percentage,
							float64(layer.info.total)/(1024*1024))
					}
				}
			}
		}
		fmt.Println() // Add final newline
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
	cmd []string,
) (string, string, error) {
	containerConfig := &container.Config{
		Image: imageName,
		Env:   env,
		ExposedPorts: nat.PortSet{
			nat.Port(port): struct{}{},
		},
	}
	
	if len(cmd) > 0 {
		containerConfig.Cmd = cmd
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
		return "", "", fmt.Errorf("failed to create container: %v", err)
	}

	bm.dbContainerId = resp.ID

	log.Println("Starting container...")
	if err := bm.dockerCli.ContainerStart(ctx, bm.dbContainerId, container.StartOptions{}); err != nil {
		return "", "", fmt.Errorf("failed to start container: %v", err)
	}
	log.Println("Container started successfully")

	if bm.debug {
		go func() {
			reader, err := bm.dockerCli.ContainerLogs(ctx, bm.dbContainerId, container.LogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Follow:     true,
			})
			if err != nil {
				log.Printf("Warning: Failed to get container logs: %v", err)
				return
			}
			defer reader.Close()

			buf := make([]byte, 1024)
			for {
				n, err := reader.Read(buf)
				if err != nil {
					if err != io.EOF {
						log.Printf("Warning: Error reading container logs: %v", err)
					}
					return
				}
				if n > 8 { // Only process if we have more than header bytes
					// Skip the first 8 bytes which contain Docker log metadata
					fmt.Print(string(buf[8:n]))
				} else if n > 0 {
					// If we have less than 8 bytes, just print everything
					fmt.Print(string(buf[:n]))
				}
			}
		}()
	}

	// Get the assigned port
	inspect, err := bm.dockerCli.ContainerInspect(ctx, bm.dbContainerId)
	if err != nil {
		return "", "", fmt.Errorf("failed to inspect container: %v", err)
	}

	bm.dbPort = inspect.NetworkSettings.Ports[nat.Port(port)][0].HostPort
	return resp.ID, bm.dbPort, nil
}

// StartContainerClient starts a client inside the container with retry logic
func (bm *BaseManager) StartContainerClient(command string, args ...string) error {
	for i := 0; i < 5; i++ {
		cmd := exec.Command("docker", append([]string{"exec", "-it", bm.dbContainerId}, append([]string{command}, args...)...)...)
		cmd.Stdin = os.Stdin
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		
		if err := cmd.Run(); err == nil {
			return nil
		}
		
		if i < 4 { // Don't sleep after last attempt
			log.Printf("Failed to connect, retrying in 5 seconds (attempt %d/5)...", i+1)
			time.Sleep(5 * time.Second)
		}
	}
	return fmt.Errorf("failed to connect after 5 attempts")
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
