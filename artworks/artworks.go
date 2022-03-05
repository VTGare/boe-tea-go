package artworks

import (
	"github.com/VTGare/boe-tea-go/models/artworks"
	"github.com/VTGare/boe-tea-go/models/guilds"
	"github.com/diamondburned/arikawa/v3/api"
)

type Provider interface {
	Match(url string) (string, bool)
	Find(id string) (Artwork, error)
	Enabled(*guilds.Guild) bool
}

type Artwork interface {
	ToModel() *artworks.ArtworkInsert
	MessageSends(footer string) ([]api.SendMessageData, error)
	URL() string
	Len() int
}
