package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

type DgraphManager struct {
	*BaseManager
}

func NewDgraphManager(dataDir string) *DgraphManager {
	base, err := NewBaseManager(dataDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &DgraphManager{
		BaseManager: base,
	}
}

func (dm *DgraphManager) StartDatabase() error {
	ctx := context.Background()

	if err := dm.PullImageIfNeeded(ctx, "dgraph/standalone:latest"); err != nil {
		return err
	}

	if err := dm.CreateContainer(ctx, "dgraph/standalone:latest", "dgraph-db", "8080/tcp", nil, "/dgraph"); err != nil {
		return err
	}

	log.Printf("Dgraph is ready and listening on port %s\n", dm.dbPort)
	return nil
}

func (dm *DgraphManager) StartClient() error {
	for i := 0; i < 5; i++ {
		cmd := exec.Command("docker", "exec", "-it", dm.dbContainerId, "dgraph-ratel")
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

func (dm *DgraphManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return dm.BaseManager.Cleanup(ctx)
}
