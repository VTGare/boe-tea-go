package messages

import "fmt"

func HelpCommandNotFound(cmd string) error {
	return newUserError(
		fmt.Sprintf(
			"Unknown command: `%v`. Please run `bt!help` to see existing commands.", cmd,
		),
	)
}
