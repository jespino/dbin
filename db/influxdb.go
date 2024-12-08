package db

import (
	"context"
	_ "embed"
	"fmt"
	"log"
	"time"
)

func init() {
	Register(DatabaseInfo{
		Name:        "influxdb",
		Description: "InfluxDB time-series database",
		Manager:     NewInfluxDBManager,
	})
}

type InfluxDBManager struct {
	*BaseManager
}

func NewInfluxDBManager(dataDir string, debug bool) DatabaseManager {
	base, err := NewBaseManager(dataDir, debug)
	if err != nil {
		panic(fmt.Sprintf("Failed to create base manager: %v", err))
	}

	return &InfluxDBManager{
		BaseManager: base,
	}
}

func (im *InfluxDBManager) StartDatabase() error {
	ctx := context.Background()

	if err := im.PullImageIfNeeded(ctx, "influxdb:latest"); err != nil {
		return err
	}

	env := []string{
		"DOCKER_INFLUXDB_INIT_MODE=setup",
		"DOCKER_INFLUXDB_INIT_USERNAME=admin",
		"DOCKER_INFLUXDB_INIT_PASSWORD=password",
		"DOCKER_INFLUXDB_INIT_ORG=myorg",
		"DOCKER_INFLUXDB_INIT_BUCKET=mybucket",
		"DOCKER_INFLUXDB_INIT_ADMIN_TOKEN=my-super-secret-auth-token",
	}

	containerId, port, err := im.CreateContainer(ctx, "influxdb:latest", "dbin-influxdb", "8086/tcp", env, "/var/lib/influxdb2", nil)
	if err != nil {
		return err
	}
	im.dbContainerId = containerId
	im.dbPort = port

	log.Printf("InfluxDB is ready and listening on port %s\n", im.dbPort)
	log.Println("\nInfluxDB Web Interface Credentials:")
	log.Println("Username: admin")
	log.Println("Password: password")
	log.Println("Organization: myorg")
	log.Println("Bucket: mybucket")
	log.Println("Token: my-super-secret-auth-token\n")
	
	return nil
}

func (im *InfluxDBManager) StartClient() error {
	return StartWebInterface(im.dbPort)
}

func (im *InfluxDBManager) Cleanup() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return im.BaseManager.Cleanup(ctx)
}
