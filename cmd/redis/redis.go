package redis

import (
	"dbin/cmd/common"
	"dbin/db"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return common.NewDatabaseCommand(common.DBCommand{
		Name:        "redis",
		Description: "Redis database",
		Manager:     db.NewRedisManager,
	})
}
