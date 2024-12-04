package main

import (
	"dbin/db"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
)

func main() {
	dataDir := flag.String("data-dir", "./data", "Directory for database data")
	flag.Parse()

	// Convert to absolute path
	absDataDir, err := filepath.Abs(*dataDir)
	if err != nil {
		log.Fatalf("Failed to get absolute path: %v", err)
	}

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(absDataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	manager := db.NewPostgresManager(absDataDir)

	if err := manager.StartDatabase(); err != nil {
		log.Fatalf("Failed to start database: %v", err)
	}

	// Set up signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start client in goroutine
	clientDone := make(chan error)
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
			log.Printf("Client error: %v", err)
		}
	case <-sigChan:
		fmt.Println("\nReceived interrupt signal")
	}

	// Cleanup
	fmt.Println("Cleaning up...")
	if err := manager.Cleanup(); err != nil {
		log.Printf("Cleanup error: %v", err)
	}
}
