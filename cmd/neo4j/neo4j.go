package neo4j

import (
	"dbin/cmd/common"
	"dbin/db"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return common.NewDatabaseCommand(common.DBCommand{
		Name:        "neo4j",
		Description: "Neo4j database",
		Manager:     db.NewNeo4jManager,
	})
}
