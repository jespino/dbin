package main

import (
	"dbin/cmd/cassandra"
	"dbin/cmd/list"
	"dbin/cmd/mariadb"
	"dbin/cmd/mongo"
	"dbin/cmd/neo4j"
	"dbin/cmd/postgres"
	"dbin/cmd/redis"
	"dbin/cmd/yugabyte"
	"log"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	cmd := &cobra.Command{
		Use:   "dbin",
		Short: "Database management tools",
		Long:  `A collection of tools for managing databases in containers`,
	}

	cmd.AddCommand(
		list.NewCommand(),
		postgres.NewCommand(),
		mongo.NewCommand(),
		cassandra.NewCommand(),
		redis.NewCommand(),
		neo4j.NewCommand(),
		mariadb.NewCommand(),
		yugabyte.NewCommand(),
	)

	if err := cmd.Execute(); err != nil {
		log.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
