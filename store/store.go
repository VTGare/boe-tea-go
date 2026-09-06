package store

import (
	"context"
	"errors"
)

type Store interface {
	ArtworkStore
	GuildStore
	UserStore
	BookmarkStore
	Init(context.Context) error
	Close(context.Context) error
}

var ErrArtworkNotFound = errors.New("artwork not found")

var ErrGuildNotFound = errors.New("guild not found")
