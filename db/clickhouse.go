package db

import (
	_ "embed"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

func init() {
	Register(DatabaseInfo{
		Name:        "clickhouse",
		Description: "ClickHouse database",
		Manager:     NewClickHouseManager,
	})
}

type ClickHouseManager struct {
	*BaseManager
}

func NewClickHouseManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &ClickHouseManager{
		BaseManager: base,
	}
}

func (chm *ClickHouseManager) StartDatabase() error {
	ctx := context.Background()

	if err := chm.PullImageIfNeeded(ctx, "clickhouse/clickhouse-server:latest"); err != nil {
		return err
	}

	env := []string{
		"CLICKHOUSE_DB=default",
		"CLICKHOUSE_USER=default",
		"CLICKHOUSE_DEFAULT_ACCESS_MANAGEMENT=1",
		"CLICKHOUSE_PASSWORD=clickhouse",
	}

	containerId, port, err := chm.CreateContainer(ctx, "clickhouse/clickhouse-server:latest", "dbin-clickhouse", "9000/tcp", env, "/var/lib/clickhouse", nil)
	if err != nil {
		return err
	}
	chm.dbContainerId = containerId
	chm.dbPort = port

	log.Printf("ClickHouse is ready and listening on port %s\n", chm.dbPort)
	return nil
}

func (chm *ClickHouseManager) StartClient() error {
	for i := 0; i < 5; i++ {
		cmd := exec.Command("docker", "exec", "-it", chm.dbContainerId, "clickhouse-client", "--password", "clickhouse")
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

func (chm *ClickHouseManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return chm.BaseManager.Cleanup(ctx)
}
