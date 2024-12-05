package postgres

import (
	"dbin/cmd/common"
	"dbin/db"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return common.NewDatabaseCommand(common.DBCommand{
		Name:        "postgres",
		Description: "PostgreSQL database",
		Manager:     db.NewPostgresManager,
	})
}
