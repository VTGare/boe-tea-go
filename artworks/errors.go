package artworks

import (
	"errors"
	"fmt"
)

// Common errors
var (
	ErrArtworkNotFound = errors.New("artwork not found")
	ErrRateLimited     = errors.New("provider rate limited")
)

type Error struct {
	provider string
	cause    error
}

func (e *Error) Error() string {
	return fmt.Sprintf("provider %v returned an error: %v", e.provider, e.cause.Error())
}

func (e *Error) Unwrap() error {
	return e.cause
}

func WrapError(p Provider, find func() (Artwork, error)) (Artwork, error) {
	artwork, err := find()
	if err != nil {
		return nil, &Error{
			provider: fmt.Sprintf("%T", p),
			cause:    err,
		}
	}

	return artwork, nil
}
