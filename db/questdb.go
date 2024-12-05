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
		Name:        "questdb",
		Description: "QuestDB database",
		Manager:     NewQuestDBManager,
	})
}

type QuestDBManager struct {
	*BaseManager
}

func NewQuestDBManager(dataDir string) DatabaseManager {
	base, err := NewBaseManager(dataDir)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &QuestDBManager{
		BaseManager: base,
	}
}

func (qm *QuestDBManager) StartDatabase() error {
	ctx := context.Background()

	if err := qm.PullImageIfNeeded(ctx, "questdb/questdb:latest"); err != nil {
		return err
	}

	if err := qm.CreateContainer(ctx, "questdb/questdb:latest", "questdb-db", "9000/tcp", nil, "/root/.questdb"); err != nil {
		return err
	}

	log.Printf("QuestDB is ready and listening on port %s\n", qm.dbPort)
	return nil
}

func (qm *QuestDBManager) StartClient() error {
	url := fmt.Sprintf("http://localhost:%s", qm.dbPort)
	log.Printf("Checking QuestDB web interface at %s", url)

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

	log.Printf("Opening QuestDB web interface at %s", url)
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

func (qm *QuestDBManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return qm.BaseManager.Cleanup(ctx)
}
