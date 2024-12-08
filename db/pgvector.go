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
		Name:        "pgvector",
		Description: "PostgreSQL with pgvector extension",
		Manager:     NewPgVectorManager,
	})
}

type PgVectorManager struct {
	*BaseManager
}

func NewPgVectorManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &PgVectorManager{
		BaseManager: base,
	}
}

func (pm *PgVectorManager) StartDatabase() error {
	ctx := context.Background()

	if err := pm.PullImageIfNeeded(ctx, "ankane/pgvector:latest"); err != nil {
		return err
	}

	env := []string{
		"POSTGRES_PASSWORD=postgres",
		"POSTGRES_USER=postgres",
		"POSTGRES_DB=postgres",
	}

	containerId, port, err := pm.CreateContainer(ctx, "ankane/pgvector:latest", "dbin-pgvector", "5432/tcp", env, "/var/lib/postgresql/data", nil)
	if err != nil {
		return err
	}
	pm.dbContainerId = containerId
	pm.dbPort = port

	fmt.Println("Waiting for database to be ready...")
	if err := pm.waitForDatabase(); err != nil {
		return fmt.Errorf("database failed to start: %v", err)
	}

	fmt.Printf("pgvector is ready and listening on port %s\n", pm.dbPort)
	return nil
}

func (pm *PgVectorManager) waitForDatabase() error {
	connStr := fmt.Sprintf("host=localhost port=%s user=postgres password=postgres dbname=postgres sslmode=disable", pm.dbPort)

	for i := 0; i < 30; i++ {
		fmt.Printf("Attempting database connection (attempt %d/30)...\n", i+1)
		db, err := sql.Open("postgres", connStr)
		if err == nil {
			err = db.Ping()
			if err == nil {
				// Enable vector extension
				_, err = db.Exec("CREATE EXTENSION IF NOT EXISTS vector")
				if err == nil {
					db.Close()
					return nil
				}
			}
			db.Close()
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("timeout waiting for database to be ready")
}

func (pm *PgVectorManager) StartClient() error {
	return pm.StartContainerClient("psql", "-U", "postgres")
}

func (pm *PgVectorManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return pm.BaseManager.Cleanup(ctx)
}
