package clickhouse

import (
	"dbin/cmd/common"
	"dbin/db"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return common.NewDatabaseCommand(common.DBCommand{
		Name:        "clickhouse",
		Description: "ClickHouse database",
		Manager:     db.NewClickHouseManager,
	})
}
