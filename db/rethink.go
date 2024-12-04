package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

type RethinkManager struct {
	*BaseManager
}

func NewRethinkManager(dataDir string) *RethinkManager {
	base, err := NewBaseManager(dataDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &RethinkManager{
		BaseManager: base,
	}
}

func (rm *RethinkManager) StartDatabase() error {
	ctx := context.Background()

	if err := rm.PullImageIfNeeded(ctx, "rethinkdb:latest"); err != nil {
		return err
	}

	if err := rm.CreateContainer(ctx, "rethinkdb:latest", "rethink-db", "28015/tcp", nil, "/data"); err != nil {
		return err
	}

	log.Printf("RethinkDB is ready and listening on port %s\n", rm.dbPort)
	return nil
}

func (rm *RethinkManager) StartClient() error {
	for i := 0; i < 5; i++ {
		cmd := exec.Command("docker", "exec", "-it", rm.dbContainerId, "rethinkdb", "connect", "localhost:28015")
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

func (rm *RethinkManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return rm.BaseManager.Cleanup(ctx)
}
