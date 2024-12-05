package db

import (
	_ "embed"
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	_ "github.com/lib/pq"
)

func init() {
	Register(DatabaseInfo{
		Name:        "postgres",
		Description: "PostgreSQL database",
		Manager:     NewPostgresManager,
	})
}

type PostgresManager struct {
	*BaseManager
}

func NewPostgresManager(dataDir string) DatabaseManager {
	base, err := NewBaseManager(dataDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &PostgresManager{
		BaseManager: base,
	}
}

func (pm *PostgresManager) StartDatabase() error {
	ctx := context.Background()

	if err := pm.PullImageIfNeeded(ctx, "postgres:latest"); err != nil {
		return err
	}

	env := []string{
		"POSTGRES_PASSWORD=postgres",
		"POSTGRES_USER=postgres",
		"POSTGRES_DB=postgres",
	}

	if err := pm.CreateContainer(ctx, "postgres:latest", "postgres-db", "5432/tcp", env, "/var/lib/postgresql/data"); err != nil {
		return err
	}

	fmt.Println("Waiting for database to be ready...")
	if err := pm.waitForDatabase(); err != nil {
		return fmt.Errorf("database failed to start: %v", err)
	}

	fmt.Printf("Database is ready and listening on port %s\n", pm.dbPort)
	return nil
}

func (pm *PostgresManager) waitForDatabase() error {
	connStr := fmt.Sprintf("host=localhost port=%s user=postgres password=postgres dbname=postgres sslmode=disable", pm.dbPort)

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

func (pm *PostgresManager) StartClient() error {
	for i := 0; i < 5; i++ {
		cmd := exec.Command("docker", "exec", "-it", pm.dbContainerId, "psql", "-U", "postgres")
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

func (pm *PostgresManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return pm.BaseManager.Cleanup(ctx)
}
