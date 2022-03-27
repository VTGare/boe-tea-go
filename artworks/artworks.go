package artworks

import (
	"github.com/VTGare/boe-tea-go/store"
	"github.com/diamondburned/arikawa/v3/api"
)

type Provider interface {
	Match(url string) (string, bool)
	Find(id string) (Artwork, error)
	Enabled(*store.Guild) bool
	Hits() int64
}

type Artwork interface {
	StoreArtwork() *store.Artwork
	MessageSends(footer string) ([]api.SendMessageData, error)
	URL() string
	Len() int
}
