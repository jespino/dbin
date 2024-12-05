package main

import (
	"dbin/cmd/list"
	"dbin/db"
	"dbin/internal/commands"
	"log"
	"os"

	"github.com/spf13/cobra"
)

var debug bool

func main() {
	cmd := &cobra.Command{
		Use:   "dbin",
		Short: "Database management tools",
		Long:  `A collection of tools for managing databases in containers`,
	}

	cmd.PersistentFlags().BoolVarP(&debug, "debug", "d", false, "Enable debug output")

	cmd.AddCommand(list.NewCommand())
	cmd.AddCommand(commands.CreateCommands(db.GetAllDatabases())...)

	if err := cmd.Execute(); err != nil {
		log.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
