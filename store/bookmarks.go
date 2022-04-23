package store

import (
	"context"
	"time"
)

type BookmarkStore interface {
	ListBookmarks(ctx context.Context, userID string, filter BookmarkFilter, order Order) ([]*Bookmark, error)
	CountBookmarks(ctx context.Context, userID string) (int64, error)
	AddBookmark(ctx context.Context, fav *Bookmark) (bool, error)
	DeleteBookmark(ctx context.Context, fav *Bookmark) (bool, error)
}

type Bookmark struct {
	UserID    string    `json:"user_id,omitempty" bson:"user_id"`
	ArtworkID int       `json:"artwork_id,omitempty" bson:"artwork_id"`
	NSFW      bool      `json:"nsfw,omitempty" bson:"nsfw"`
	CreatedAt time.Time `json:"created_at,omitempty" bson:"created_at"`
}

type BookmarkFilter int

const (
	BookmarkFilterAll BookmarkFilter = iota
	BookmarkFilterSafe
	BookmarkFilterUnsafe
)
