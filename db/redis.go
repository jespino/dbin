package db

import (
	_ "embed"
	"context"
	"fmt"
	"log"
	"time"
)

func init() {
	Register(DatabaseInfo{
		Name:        "redis",
		Description: "Redis database",
		Manager:     NewRedisManager,
	})
}

type RedisManager struct {
	*BaseManager
}

func NewRedisManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
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

	containerId, port, err := rm.CreateContainer(ctx, "redis:latest", "dbin-redis", "6379/tcp", nil, "/data", nil)
	if err != nil {
		return err
	}
	rm.dbContainerId = containerId
	rm.dbPort = port
		return err
	}

	log.Printf("Redis is ready and listening on port %s\n", rm.dbPort)
	return nil
}

func (rm *RedisManager) StartClient() error {
	return rm.StartContainerClient("redis-cli")
}

func (rm *RedisManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return rm.BaseManager.Cleanup(ctx)
}
