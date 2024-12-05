package db

import (
	_ "embed"
	"context"
	"fmt"
	"log"
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
