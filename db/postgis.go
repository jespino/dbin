package db

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"time"
	"database/sql"
	_ "github.com/lib/pq"
)

func init() {
	Register(DatabaseInfo{
		Name:        "postgis",
		Description: "PostGIS spatial database",
		Manager:     NewPostGISManager,
	})
}

type PostGISManager struct {
	*BaseManager
}

func NewPostGISManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &PostGISManager{
		BaseManager: base,
	}
}

func (pm *PostGISManager) StartDatabase() error {
	ctx := context.Background()

	if err := pm.PullImageIfNeeded(ctx, "postgis/postgis:latest"); err != nil {
		return err
	}

	env := []string{
		"POSTGRES_PASSWORD=postgres",
		"POSTGRES_USER=postgres",
		"POSTGRES_DB=postgres",
	}

	containerId, port, err := pm.CreateContainer(ctx, "postgis/postgis:latest", "dbin-postgis", "5432/tcp", env, "/var/lib/postgresql/data", nil)
	if err != nil {
		return err
	}
	pm.dbContainerId = containerId
	pm.dbPort = port

	fmt.Println("Waiting for database to be ready...")
	if err := pm.waitForDatabase(); err != nil {
		return fmt.Errorf("database failed to start: %v", err)
	}

	fmt.Printf("PostGIS is ready and listening on port %s\n", pm.dbPort)
	return nil
}

func (pm *PostGISManager) waitForDatabase() error {
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

func (pm *PostGISManager) StartClient() error {
	return pm.StartContainerClient("psql", "-U", "postgres")
}

func (pm *PostGISManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return pm.BaseManager.Cleanup(ctx)
}
