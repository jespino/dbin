package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

type SurrealManager struct {
	*BaseManager
}

func NewSurrealManager(dataDir string) *SurrealManager {
	base, err := NewBaseManager(dataDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &SurrealManager{
		BaseManager: base,
	}
}

func (sm *SurrealManager) StartDatabase() error {
	ctx := context.Background()

	if err := sm.PullImageIfNeeded(ctx, "surrealdb/surrealdb:latest"); err != nil {
		return err
	}

	env := []string{
		"SURREAL_USER=root",
		"SURREAL_PASS=root",
	}

	if err := sm.CreateContainer(ctx, "surrealdb/surrealdb:latest", "surreal-db", "8000/tcp", env, "/data"); err != nil {
		return err
	}

	log.Printf("SurrealDB is ready and listening on port %s\n", sm.dbPort)
	return nil
}

func (sm *SurrealManager) StartClient() error {
	for i := 0; i < 5; i++ {
		cmd := exec.Command("docker", "exec", "-it", sm.dbContainerId, "surreal", "sql", "-u", "root", "-p", "root")
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

func (sm *SurrealManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return sm.BaseManager.Cleanup(ctx)
}
