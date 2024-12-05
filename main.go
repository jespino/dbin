package main

import (
	"dbin/cmd/list"
	"dbin/db"
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

	cmd.AddCommand(list.NewCommand())
	cmd.AddCommand(db.CreateCommands()...)

	if err := cmd.Execute(); err != nil {
		log.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
