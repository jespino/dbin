package mongo

import (
	"dbin/cmd/common"
	"dbin/db"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return common.NewDatabaseCommand(common.DBCommand{
		Name:        "mongo",
		Description: "MongoDB database",
		Manager:     db.NewMongoManager,
	})
}
