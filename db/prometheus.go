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
		Name:        "prometheus",
		Description: "Prometheus monitoring system",
		Manager:     NewPrometheusManager,
	})
}

type PrometheusManager struct {
	*BaseManager
}

func NewPrometheusManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
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
	return StartWebInterface(pm.dbPort)
}

func (pm *PrometheusManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return pm.BaseManager.Cleanup(ctx)
}
