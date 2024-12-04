package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

type RedisManager struct {
	*BaseManager
}

func NewRedisManager(dataDir string) *RedisManager {
	base, err := NewBaseManager(dataDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &RedisManager{
		BaseManager: base,
	}
}

func (rm *RedisManager) StartDatabase() error {
	ctx := context.Background()

	if err := rm.PullImageIfNeeded(ctx, "redis:latest"); err != nil {
		return err
	}

	if err := rm.CreateContainer(ctx, "redis:latest", "redis-db", "6379/tcp", nil, "/data"); err != nil {
		return err
	}

	log.Printf("Redis is ready and listening on port %s\n", rm.dbPort)
	return nil
}

func (rm *RedisManager) StartClient() error {
	for i := 0; i < 5; i++ {
		cmd := exec.Command("docker", "exec", "-it", rm.dbContainerId, "redis-cli")
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

func (rm *RedisManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return rm.BaseManager.Cleanup(ctx)
}
