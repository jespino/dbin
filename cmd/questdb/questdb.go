package questdb

import (
	"dbin/cmd/common"
	"dbin/db"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return common.NewDatabaseCommand(common.DBCommand{
		Name:        "questdb",
		Description: "QuestDB database",
		Manager:     db.NewQuestDBManager,
	})
}
