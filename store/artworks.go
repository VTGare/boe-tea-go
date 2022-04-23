package store

import (
	"context"
	"time"
)

type ArtworkStore interface {
	Artwork(ctx context.Context, id int, url string) (*Artwork, error)
	CreateArtwork(context.Context, *Artwork) (*Artwork, error)
	SearchArtworks(context.Context, ArtworkFilter, ...ArtworkSearchOptions) ([]*Artwork, error)
}

type Artwork struct {
	ID         int       `json:"id" bson:"artwork_id"`
	Title      string    `json:"title" bson:"title"`
	Author     string    `json:"author" bson:"author"`
	URL        string    `json:"url" bson:"url"`
	Images     []string  `json:"images" bson:"images"`
	Favourites int       `json:"favourites" bson:"favourites"`
	CreatedAt  time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt  time.Time `json:"updated_at" bson:"updated_at"`
}

type Order int

const (
	Descending Order = iota - 1
	_
	Ascending
)

type ArtworkSort int

const (
	ByTime ArtworkSort = iota
	ByFavourites
)

func (s ArtworkSort) String() string {
	return map[ArtworkSort]string{
		ByTime:       "created_at",
		ByFavourites: "favourites",
	}[s]
}

type ArtworkSearchOptions struct {
	Limit int64
	Page  int64
	Order Order
	Sort  ArtworkSort
}

type ArtworkFilter struct {
	IDs    []int  `query:"id"`
	Title  string `query:"title"`
	Author string `query:"author"`
	Query  string `query:"query"`
	URL    string `query:"url"`
	Time   time.Duration
}

func DefaultSearchOptions() ArtworkSearchOptions {
	return ArtworkSearchOptions{
		Limit: 100,
		Page:  0,
		Order: Descending,
		Sort:  ByTime,
	}
}
