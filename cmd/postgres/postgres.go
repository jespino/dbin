package postgres

import (
	"dbin/db"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
)

var dataDir string

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "postgres",
		Short: "Start a PostgreSQL database instance",
		Long:  `Start a PostgreSQL database instance in a Docker container with an interactive psql client`,
		RunE:  run,
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "./data", "Directory for database data")
	return cmd
}

func run(cmd *cobra.Command, args []string) error {
	// Convert to absolute path
	absDataDir, err := filepath.Abs(dataDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %v", err)
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(absDataDir, 0755); err != nil {
		return fmt.Errorf("failed to create data directory: %v", err)
	}

	manager := db.NewPostgresManager(absDataDir)

	// Ensure cleanup happens no matter how we exit
	defer func() {
		fmt.Println("\nCleaning up...")
		if err := manager.Cleanup(); err != nil {
			log.Printf("Cleanup error: %v", err)
		}
	}()

	if err := manager.StartDatabase(); err != nil {
		return fmt.Errorf("failed to start database: %v", err)
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start client in goroutine
	clientDone := make(chan error, 1)
	go func() {
		if err := manager.StartClient(); err != nil {
			clientDone <- err
			return
		}
		clientDone <- nil
	}()

	// Wait for either client to finish or interrupt signal
	select {
	case err := <-clientDone:
		if err != nil {
			return fmt.Errorf("client error: %v", err)
		}
	case <-sigChan:
		return fmt.Errorf("interrupted")
	}

	return nil
}
