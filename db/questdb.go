package db

import (
	_ "embed"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"
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

	if err := qm.CreateContainer(ctx, "questdb/questdb:latest", "questdb-db", "8812/tcp", nil, "/root/.questdb"); err != nil {
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
	
	// Keep the container running
	select {} // Block forever
}

func (qm *QuestDBManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return qm.BaseManager.Cleanup(ctx)
}
