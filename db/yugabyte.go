package db

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
)

type YugabyteManager struct {
	*BaseManager
}

func NewYugabyteManager(dataDir string) *YugabyteManager {
	base, err := NewBaseManager(dataDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &YugabyteManager{
		BaseManager: base,
	}
}

func (ym *YugabyteManager) StartDatabase() error {
	ctx := context.Background()

	if err := ym.PullImageIfNeeded(ctx, "yugabytedb/yugabyte:latest"); err != nil {
		return err
	}

	env := []string{
		"YSQL_USER=yugabyte",
		"YSQL_PASSWORD=yugabyte",
		"YSQL_DB=yugabyte",
	}

	if err := ym.CreateContainer(ctx, "yugabytedb/yugabyte:latest", "yugabyte-db", "5433/tcp", env, "/home/yugabyte/yb_data"); err != nil {
		return err
	}

	fmt.Println("Waiting for database to be ready...")
	if err := ym.waitForDatabase(); err != nil {
		return fmt.Errorf("database failed to start: %v", err)
	}

	fmt.Printf("YugabyteDB is ready and listening on port %s\n", ym.dbPort)
	return nil
}

func (ym *YugabyteManager) waitForDatabase() error {
	for i := 0; i < 30; i++ {
		fmt.Printf("Checking database status (attempt %d/30)...\n", i+1)
		cmd := exec.Command("docker", "exec", ym.dbContainerId, "ysqlsh", "-U", "yugabyte", "-d", "yugabyte", "-c", "SELECT 1")
		if err := cmd.Run(); err == nil {
			return nil
		}
		time.Sleep(2 * time.Second)
	}
	return fmt.Errorf("timeout waiting for database to be ready")
}

func (ym *YugabyteManager) StartClient() error {
	for i := 0; i < 5; i++ {
		cmd := exec.Command("docker", "exec", "-it", ym.dbContainerId, "ysqlsh", "-U", "yugabyte", "-d", "yugabyte")
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

func (ym *YugabyteManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return ym.BaseManager.Cleanup(ctx)
}
