package neo4j

import (
	"dbin/db"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/spf13/cobra"
)

var dataDir string

func NewCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "neo4j",
		Short: "Start a Neo4j instance",
		Long:  `Start a Neo4j instance in a Docker container with an interactive cypher-shell client`,
		RunE:  run,
	}

	cmd.Flags().StringVar(&dataDir, "data-dir", "./data", "Directory for database data")
	return cmd
}

func run(cmd *cobra.Command, args []string) error {
	var absDataDir string
	if dataDir != "./data" { // Only process if explicitly set
		var err error
		absDataDir, err = filepath.Abs(dataDir)
		if err != nil {
			return fmt.Errorf("failed to get absolute path: %v", err)
		}

		// Create data directory if it doesn't exist
		if err := os.MkdirAll(absDataDir, 0755); err != nil {
			return fmt.Errorf("failed to create data directory: %v", err)
		}
		log.Printf("Starting Neo4j manager with data directory: %s", absDataDir)
	} else {
		log.Println("Starting Neo4j manager with ephemeral storage")
	}

	manager := db.NewNeo4jManager(absDataDir)

	log.Println("Initializing database...")
	if err := manager.StartDatabase(); err != nil {
		return fmt.Errorf("failed to start database: %v", err)
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Starting cypher-shell client...")
	// Start client in goroutine
	clientDone := make(chan error, 1)
	go func() {
		if err := manager.StartClient(); err != nil {
			log.Printf("Client error: %v", err)
			clientDone <- err
			return
		}
		clientDone <- nil
	}()

	// Create WaitGroup for cleanup coordination
	var wg sync.WaitGroup
	wg.Add(1)

	// Wait for either client to finish or interrupt signal
	var result error
	select {
	case err := <-clientDone:
		if err != nil {
			result = fmt.Errorf("client error: %v", err)
		}
		// Clean up after client exits normally
		log.Println("Client exited, starting cleanup...")
		go func() {
			defer wg.Done()
			if err := manager.Cleanup(); err != nil {
				log.Printf("Cleanup error: %v", err)
			}
			log.Println("Cleanup completed")
		}()
	case <-sigChan:
		// Stop the database container immediately on interrupt
		log.Println("Received interrupt signal, starting cleanup...")
		go func() {
			defer wg.Done()
			if err := manager.Cleanup(); err != nil {
				log.Printf("Cleanup error: %v", err)
			}
			log.Println("Cleanup completed")
		}()
		result = fmt.Errorf("interrupted")
	}

	// Wait for cleanup to complete
	wg.Wait()
	return result
}
