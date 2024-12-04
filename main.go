package main

import (
	"dbin/cmd/cassandra"
	"dbin/cmd/mongo"
	"dbin/cmd/postgres"
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
		postgres.NewCommand(),
		mongo.NewCommand(),
		cassandra.NewCommand(),
	)

	if err := cmd.Execute(); err != nil {
		log.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
