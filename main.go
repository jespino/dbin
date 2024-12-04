package main

import (
	"dbin/cmd/cassandra"
	"dbin/cmd/list"
	"dbin/cmd/mongo"
	"dbin/cmd/neo4j"
	"dbin/cmd/postgres"
	"dbin/cmd/redis"
	"dbin/cmd/rethink"
	"dbin/cmd/surreal"
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
		rethink.NewCommand(),
		surreal.NewCommand(),
	)

	if err := cmd.Execute(); err != nil {
		log.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
