package router

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Sentinel errors returned by the router.
var (
	ErrMissingOption     = errors.New("missing required option")
	ErrMissingSubcommand = errors.New("missing subcommand")
	ErrUnknownSubcommand = errors.New("unknown subcommand")
	ErrNoHandler         = errors.New("command has no handler")
	ErrNoResponse        = errors.New("no response has been sent yet")
	ErrMissingTarget     = errors.New("missing command target")
)

// UserFacing errors are safe to show the invoker; anything else gets a
// generic "something went wrong" reply.
type UserFacing interface {
	error
	UserMessage() string
}

func UserMessageOf(err error) (string, bool) {
	if uf, ok := errors.AsType[UserFacing](err); ok {
		return uf.UserMessage(), true
	}

	return "", false
}

// UserError is for when the user did something wrong (bad input, nothing found).
type UserError struct {
	Message string
	Err     error
}

func NewUserError(message string) *UserError { return &UserError{Message: message} }

func Errorf(format string, args ...any) *UserError {
	return &UserError{Message: fmt.Sprintf(format, args...)}
}

func WrapUserError(message string, err error) *UserError {
	return &UserError{Message: message, Err: err}
}

func (e *UserError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

func (e *UserError) Unwrap() error       { return e.Err }
func (e *UserError) UserMessage() string { return e.Message }

type CheckError struct {
	Check   string
	Message string

	// Silent sends no reply; for commands you don't want to leak.
	Silent bool
}

func (e *CheckError) Error() string {
	return fmt.Sprintf("check %q failed: %s", e.Check, e.Message)
}

func (e *CheckError) UserMessage() string { return e.Message }

type CooldownError struct {
	Remaining time.Duration
	Scope     CooldownScope
}

func (e *CooldownError) Error() string {
	return fmt.Sprintf("on cooldown (%s) for %s", e.Scope, e.Remaining)
}

func (e *CooldownError) UserMessage() string {
	return fmt.Sprintf("This command is on cooldown. Try again in %s.", humanDuration(e.Remaining))
}

type OptionError struct {
	Option *Option
	Value  string
	Err    error
}

func (e *OptionError) Error() string {
	return fmt.Sprintf("option %q: %v", e.Option.Name, e.Err)
}

func (e *OptionError) Unwrap() error { return e.Err }

func (e *OptionError) UserMessage() string {
	if errors.Is(e.Err, ErrMissingOption) {
		return fmt.Sprintf("Missing required argument `%s`.", e.Option.Name)
	}

	if e.Value != "" {
		return fmt.Sprintf("`%s` is not a valid value for `%s`: %s.", e.Value, e.Option.Name, e.Err)
	}

	return fmt.Sprintf("Invalid value for `%s`: %s.", e.Option.Name, e.Err)
}

type SubcommandError struct {
	Command *Command
	Given   string
}

func (e *SubcommandError) Error() string {
	if e.Given != "" {
		return fmt.Sprintf("%s: unknown subcommand %q", e.Command.QualifiedName(), e.Given)
	}
	return fmt.Sprintf("%s: missing subcommand", e.Command.QualifiedName())
}

// Unwrap matches ErrUnknownSubcommand or ErrMissingSubcommand.
func (e *SubcommandError) Unwrap() error {
	if e.Given != "" {
		return ErrUnknownSubcommand
	}
	return ErrMissingSubcommand
}

func (e *SubcommandError) UserMessage() string {
	names := make([]string, 0, len(e.Command.Subcommands))
	for _, sub := range e.Command.Subcommands {
		if !sub.Hidden {
			names = append(names, "`"+sub.Name+"`")
		}
	}

	if e.Given != "" {
		return fmt.Sprintf("Unknown subcommand `%s`. Available: %s.", e.Given, strings.Join(names, ", "))
	}
	return fmt.Sprintf("Missing subcommand. Available: %s.", strings.Join(names, ", "))
}

type PanicError struct {
	Value any
	Stack []byte
}

func (e *PanicError) Error() string { return fmt.Sprintf("panic: %v", e.Value) }

func humanDuration(d time.Duration) string {
	if d < time.Second {
		return "a moment"
	}

	return d.Round(time.Second).String()
}
