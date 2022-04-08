package messages

import (
	"fmt"

	"github.com/VTGare/gumi"
)

type IncorrectCmd struct {
	Name        string
	Usage       string
	Example     string
	Description string
	Embed       *CommandHelp
}

func (cmd *IncorrectCmd) Error() string {
	return fmt.Sprintf("Command `%v` was used incorrectly", cmd.Name)
}

func ErrIncorrectCmd(cmd *gumi.Command) error {
	return &IncorrectCmd{
		Name:        cmd.Name,
		Usage:       cmd.Usage,
		Example:     cmd.Example,
		Description: cmd.Description,
		Embed: &CommandHelp{
			Usage:   "Usage",
			Example: "Example",
		},
	}
}

type UserErr struct {
	msg string
	err error
}

func (ue *UserErr) Error() string {
	return ue.msg
}

func (ue *UserErr) Unwrap() error {
	return ue.err
}

func newUserError(msg string, errs ...error) *UserErr {
	var err error
	if len(errs) > 0 {
		err = errs[0]
	}

	return &UserErr{
		msg: msg,
		err: err,
	}
}

func ErrInsertGroup(group, parent string) error {
	return newUserError(fmt.Sprintf(
		"Couldn't create a new group. One of the following is true:\n%v\n%v",
		"• Group named `"+group+"` already exist;",
		"• Channel `"+parent+"` is a parent of another group.",
	))
}

func ErrDeleteGroup(group string) error {
	return newUserError(fmt.Sprintf(
		"Couldn't delete group `%v`. Group doesn't exist.",
		group,
	))
}

func ErrGuildNotFound(err error, id string) error {
	return newUserError(
		fmt.Sprintf("Failed to fetch guild information from the database. Guild ID: %v", id),
		err,
	)
}

func ErrUserNotFound(err error, id string) error {
	return newUserError(
		fmt.Sprintf("Failed to fetch user profile from the database. User ID: %v", id),
		err,
	)
}

func ErrChannelNotFound(err error, id string) error {
	return newUserError(
		fmt.Sprintf(
			"Channel with ID `%v` couldn't be fetched. Please make sure Boe Tea has access to it. If you believe it's a mistake please report it to the dev `bt!feedback`",
			id,
		),
		err,
	)
}

func ErrSkipIndexSyntax(str string) error {
	return newUserError(
		fmt.Sprintf(
			"Argument `%v` couldn't be parsed correctly. It's neither a numberic range nor an integer. %v",
			str,
			"Please follow one of the following syntaxes:\n• A numeric range (e.g. 1-5)\n• An integer number (e.g. 177013)",
		),
	)
}
