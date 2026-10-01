package messages

import "fmt"

func ErrParseBool(value string) error {
	return newUserError(fmt.Sprintf("Failed to parse %v to boolean", value))
}
