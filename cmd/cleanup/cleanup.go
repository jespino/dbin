package cleanup

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up all dbin containers and networks",
		Long:  `Remove all Docker containers and networks created by dbin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cleanup()
		},
	}
}

func cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithVersion("1.46"),
	)
	if err != nil {
		return fmt.Errorf("failed to create Docker client: %v", err)
	}

	// List all containers
	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		return fmt.Errorf("failed to list containers: %v", err)
	}

	// Stop and remove containers with dbin- prefix
	for _, c := range containers {
		for _, name := range c.Names {
			if strings.HasPrefix(name, "/dbin-") {
				log.Printf("Stopping container %s...", name)
				if err := cli.ContainerStop(ctx, c.ID, container.StopOptions{}); err != nil {
					log.Printf("Warning: Failed to stop container %s: %v", name, err)
				}
				log.Printf("Removing container %s...", name)
				if err := cli.ContainerRemove(ctx, c.ID, container.RemoveOptions{Force: true}); err != nil {
					log.Printf("Warning: Failed to remove container %s: %v", name, err)
				}
			}
		}
	}

	// List all networks
	networks, err := cli.NetworkList(ctx, types.NetworkListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list networks: %v", err)
	}

	// Remove networks with dbin- prefix
	for _, network := range networks {
		if strings.HasPrefix(network.Name, "dbin-") {
			log.Printf("Removing network %s...", network.Name)
			if err := cli.NetworkRemove(ctx, network.ID); err != nil {
				log.Printf("Warning: Failed to remove network %s: %v", network.Name, err)
			}
		}
	}

	log.Println("Cleanup completed")
	return nil
}
