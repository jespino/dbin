package db

import (
	"context"
	_ "embed"
	"fmt"
	"time"
	"database/sql"
	_ "github.com/lib/pq"
)

func init() {
	Register(DatabaseInfo{
		Name:        "timescale",
		Description: "TimescaleDB time-series database",
		Manager:     NewTimescaleManager,
	})
}

type TimescaleManager struct {
	*BaseManager
}

func NewTimescaleManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &TimescaleManager{
		BaseManager: base,
	}
}

func (tm *TimescaleManager) StartDatabase() error {
	ctx := context.Background()

	if err := tm.PullImageIfNeeded(ctx, "timescale/timescaledb:latest-pg15"); err != nil {
		return err
	}

	env := []string{
		"POSTGRES_PASSWORD=postgres",
		"POSTGRES_USER=postgres",
		"POSTGRES_DB=postgres",
	}

	containerId, port, err := tm.CreateContainer(ctx, "timescale/timescaledb:latest-pg15", "dbin-timescale", "5432/tcp", env, "/var/lib/postgresql/data", nil)
	if err != nil {
		return err
	}
	tm.dbContainerId = containerId
	tm.dbPort = port

	fmt.Println("Waiting for database to be ready...")
	if err := tm.waitForDatabase(); err != nil {
		return fmt.Errorf("database failed to start: %v", err)
	}

	fmt.Printf("TimescaleDB is ready and listening on port %s\n", tm.dbPort)
	return nil
}

func (tm *TimescaleManager) waitForDatabase() error {
	connStr := fmt.Sprintf("host=localhost port=%s user=postgres password=postgres dbname=postgres sslmode=disable", tm.dbPort)

	for i := 0; i < 30; i++ {
		fmt.Printf("Attempting database connection (attempt %d/30)...\n", i+1)
		db, err := sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
			if err == nil {
				db.Close()
				return nil
			}
			db.Close()
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for database to be ready")
}

func (tm *TimescaleManager) StartClient() error {
	return tm.StartContainerClient("psql", "-U", "postgres")
}

func (tm *TimescaleManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return tm.BaseManager.Cleanup(ctx)
}
