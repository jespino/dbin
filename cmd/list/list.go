package list

import (
	"fmt"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List supported databases",
		Long:  `Display a list of all databases supported by dbin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("Supported databases:")
			fmt.Println("- PostgreSQL (postgres)")
			fmt.Println("- MongoDB (mongo)")
			fmt.Println("- Cassandra (cassandra)")
			fmt.Println("- Redis (redis)")
			fmt.Println("- Neo4j (neo4j)")
			fmt.Println("- MariaDB (mariadb)")
			fmt.Println("- ClickHouse (clickhouse)")
			fmt.Println("- QuestDB (questdb)")
			return nil
		},
	}
}
