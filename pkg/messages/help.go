package messages

import "fmt"

func HelpCommandNotFound(cmd string) error {
	return newUserError(
		fmt.Sprintf(
			"Command `%v` doesn't exist.", cmd,
		),
	)
}
