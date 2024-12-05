package list

import (
	"fmt"
	"sort"

	"dbin/db"

	"github.com/spf13/cobra"
)

func NewCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List supported databases",
		Long:  `Display a list of all databases supported by dbin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			databases := db.GetAllDatabases()
			
			// Sort databases by name for consistent output
			sort.Slice(databases, func(i, j int) bool {
				return databases[i].Name < databases[j].Name
			})

			fmt.Println("Supported databases:")
			for _, info := range databases {
				fmt.Printf("- %s (%s)\n", info.Description, info.Name)
			}
			return nil
		},
	}
}
