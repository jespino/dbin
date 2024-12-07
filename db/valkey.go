package db

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"time"
)

func init() {
	Register(DatabaseInfo{
		Name:        "valkey",
		Description: "ValKey key-value store",
		Manager:     NewValKeyManager,
	})
}

type ValKeyManager struct {
	*BaseManager
}

func NewValKeyManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &ValKeyManager{
		BaseManager: base,
	}
}

func (vk *ValKeyManager) StartDatabase() error {
	ctx := context.Background()

	if err := vk.PullImageIfNeeded(ctx, "valkey/valkey:latest"); err != nil {
		return err
	}

	env := []string{
		"VALKEY_USER=admin",
		"VALKEY_PASSWORD=password",
	}

	containerId, port, err := vk.CreateContainer(ctx, "valkey/valkey:latest", "dbin-valkey", "6380/tcp", env, "/data", nil)
	if err != nil {
		return err
	}
	vk.dbContainerId = containerId
	vk.dbPort = port

	log.Printf("ValKey is ready and listening on port %s\n", vk.dbPort)
	return nil
}

func (vk *ValKeyManager) StartClient() error {
	return vk.StartContainerClient("valkey-cli", "-n", "0")
}

func (vk *ValKeyManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return vk.BaseManager.Cleanup(ctx)
}
