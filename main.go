package main

import (
	"dbin/db"
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {
	dataDir := flag.String("data-dir", "./data", "Directory for database data")
	port := flag.Int("port", 5432, "Port for PostgreSQL")
	flag.Parse()

	// Create data directory if it doesn't exist
	if err := os.MkdirAll(*dataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	manager := db.NewPostgresManager(*dataDir, *port)

	if err := manager.StartDatabase(); err != nil {
		log.Fatalf("Failed to start database: %v", err)
	}

	fmt.Println("Database is running. Press Ctrl+C to stop.")
	
	// Wait for interrupt signal
	select {}
}
