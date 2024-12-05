package mariadb

import (
	"dbin/cmd/common"
	"dbin/db"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return common.NewDatabaseCommand(common.DBCommand{
		Name:        "mariadb",
		Description: "MariaDB database",
		Manager:     db.NewMariaDBManager,
	})
}
