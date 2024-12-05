package db

import (
	_ "embed"
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"
	"golang.org/x/term"
)

func init() {
	Register(DatabaseInfo{
		Name:        "prometheus",
		Description: "Prometheus monitoring system",
		Manager:     NewPrometheusManager,
	})
}

type PrometheusManager struct {
	*BaseManager
}

func NewPrometheusManager(dataDir string) DatabaseManager {
	base, err := NewBaseManager(dataDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &PrometheusManager{
		BaseManager: base,
	}
}

func (pm *PrometheusManager) StartDatabase() error {
	ctx := context.Background()

	if err := pm.PullImageIfNeeded(ctx, "prom/prometheus:latest"); err != nil {
		return err
	}

	if err := pm.CreateContainer(ctx, "prom/prometheus:latest", "prometheus-db", "9090/tcp", nil, "/prometheus"); err != nil {
		return err
	}

	log.Printf("Prometheus is ready and listening on port %s\n", pm.dbPort)
	return nil
}

func (pm *PrometheusManager) StartClient() error {
	url := fmt.Sprintf("http://localhost:%s", pm.dbPort)
	log.Printf("Checking Prometheus web interface at %s", url)

	// Check if server is responding
	for i := 0; i < 5; i++ {
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == 200 {
			resp.Body.Close()
			break
		}
		if err != nil {
			log.Printf("Server not ready (attempt %d/5): %v", i+1, err)
		} else {
			resp.Body.Close()
			log.Printf("Server returned status %d (attempt %d/5)", resp.StatusCode, i+1)
		}
		if i < 4 {
			time.Sleep(5 * time.Second)
		} else {
			return fmt.Errorf("server failed to respond after 5 attempts")
		}
	}

	log.Printf("Opening Prometheus web interface at %s", url)
	cmd := exec.Command("xdg-open", url)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to open browser: %v", err)
	}
	
	fmt.Println("\nPress 'q' to exit...")
	
	// Get the current state of the terminal
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("failed to set terminal to raw mode: %v", err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	buffer := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(buffer)
		if err != nil {
			return fmt.Errorf("error reading input: %v", err)
		}
		
		if buffer[0] == 'q' {
			fmt.Println() // Add newline after 'q'
			log.Println("Shutting down...")
			return nil
		}
	}
}

func (pm *PrometheusManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return pm.BaseManager.Cleanup(ctx)
}
