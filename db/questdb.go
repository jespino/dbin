package db

import (
	_ "embed"
	"context"
	"fmt"
	"log"
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

func NewQuestDBManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
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

	containerId, port, err := qm.CreateContainer(ctx, "questdb/questdb:latest", "dbin-questdb", "9000/tcp", nil, "/root/.questdb", nil)
	if err != nil {
		return err
	}
	qm.dbContainerId = containerId
	qm.dbPort = port

	log.Printf("QuestDB is ready and listening on port %s\n", qm.dbPort)
	return nil
}

func (qm *QuestDBManager) StartClient() error {
	return StartWebInterface(qm.dbPort)
}

func (qm *QuestDBManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return qm.BaseManager.Cleanup(ctx)
}
