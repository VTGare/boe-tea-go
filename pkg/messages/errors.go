package messages

import (
	"fmt"

	"github.com/VTGare/gumi"
)

type IncorrectCmd struct {
	Name    string
	Usage   string
	Example string
}

func (cmd *IncorrectCmd) Error() string {
	return fmt.Sprintf("Command `%v` was used incorrectly", cmd.Name)
}

func ErrIncorrectCmd(cmd *gumi.Command) error {
	return &IncorrectCmd{
		Name:    cmd.Name,
		Usage:   cmd.Usage,
		Example: cmd.Example,
	}
}

func ErrGuildNotFound(id string) error {
	return fmt.Errorf("Failed to fetch guild information from the database. Guild ID: %v", id)
}
