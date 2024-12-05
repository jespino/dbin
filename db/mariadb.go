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

	_ "github.com/go-sql-driver/mysql"
)

func init() {
	Register(DatabaseInfo{
		Name:        "mariadb",
		Description: "MariaDB database",
		Manager:     NewMariaDBManager,
	})
}

type MariaDBManager struct {
	*BaseManager
}

func NewMariaDBManager(dataDir string) DatabaseManager {
	base, err := NewBaseManager(dataDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &MariaDBManager{
		BaseManager: base,
	}
}

func (mm *MariaDBManager) StartDatabase() error {
	ctx := context.Background()

	if err := mm.PullImageIfNeeded(ctx, "mariadb:latest"); err != nil {
		return err
	}

	env := []string{
		"MYSQL_ROOT_PASSWORD=root",
		"MYSQL_DATABASE=test",
	}

	if err := mm.CreateContainer(ctx, "mariadb:latest", "mariadb-db", "3306/tcp", env, "/var/lib/mysql"); err != nil {
		return err
	}

	fmt.Println("Waiting for database to be ready...")
	if err := mm.waitForDatabase(); err != nil {
		return fmt.Errorf("database failed to start: %v", err)
	}

	fmt.Printf("Database is ready and listening on port %s\n", mm.dbPort)
	return nil
}

func (mm *MariaDBManager) waitForDatabase() error {
	connStr := fmt.Sprintf("root:root@tcp(localhost:%s)/test", mm.dbPort)

	for i := 0; i < 30; i++ {
		fmt.Printf("Attempting database connection (attempt %d/30)...\n", i+1)
		db, err := sql.Open("mysql", connStr)
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

func (mm *MariaDBManager) StartClient() error {
	for i := 0; i < 5; i++ {
		cmd := exec.Command("docker", "exec", "-it", mm.dbContainerId, "mariadb", "-uroot", "-proot")
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

func (mm *MariaDBManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return mm.BaseManager.Cleanup(ctx)
}
