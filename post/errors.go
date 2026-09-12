package post

import (
	"errors"
	"net/http"

	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/bwmarrin/discordgo"
)

type Kind int

const (
	KindUnknown Kind = iota

	KindNoPerms
	KindTransient
)

type Error struct {
	Kind  Kind
	Cause error
}

func (e *Error) Error() string {
	return e.Cause.Error()
}

func (e *Error) Unwrap() error {
	return e.Cause
}

// renderError marks a fatal artwork-render failure. Unlike per-message
// send failures it aborts the run instead of joining and continuing.
type renderError struct {
	err error
}

func (e *renderError) Error() string {
	return e.err.Error()
}

func (e *renderError) Unwrap() error {
	return e.err
}

func classify(err error) Kind {
	if errors.Is(err, sender.ErrSkipped) {
		return KindNoPerms
	}

	var restErr *discordgo.RESTError
	if errors.As(err, &restErr) && restErr.Response != nil {
		switch restErr.Response.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return KindNoPerms
		}
	}

	return KindTransient
}
