package db

import (
	"fmt"
	"dbin/cmd/common"

	"github.com/spf13/cobra"
)

type DatabaseInfo struct {
	Name        string
	Description string
	Manager     func(string) DatabaseManager
}

var registry = make(map[string]DatabaseInfo)

func Register(info DatabaseInfo) {
	registry[info.Name] = info
}

func GetDatabaseInfo(name string) (DatabaseInfo, error) {
	if info, exists := registry[name]; exists {
		return info, nil
	}
	return DatabaseInfo{}, fmt.Errorf("database %s not found in registry", name)
}

func GetAllDatabases() []DatabaseInfo {
	var databases []DatabaseInfo
	for _, info := range registry {
		databases = append(databases, info)
	}
	return databases
}

func CreateCommands() []*cobra.Command {
	var commands []*cobra.Command
	for _, info := range registry {
		cmd := common.NewDatabaseCommand(common.DBCommand{
			Name:        info.Name,
			Description: info.Description,
			Manager:     info.Manager,
		})
		commands = append(commands, cmd)
	}
	return commands
}
