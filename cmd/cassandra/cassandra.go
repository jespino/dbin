package cassandra

import (
	"dbin/cmd/common"
	"dbin/db"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return common.NewDatabaseCommand(common.DBCommand{
		Name:        "cassandra",
		Description: "Cassandra database",
		Manager:     db.NewCassandraManager,
	})
}
