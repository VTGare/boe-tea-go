package artworks

import (
	"github.com/VTGare/boe-tea-go/store"
	"github.com/bwmarrin/discordgo"
)

type Provider interface {
	Match(url string) (string, bool)
	Find(id string) (Artwork, error)
	Enabled(*store.Guild) bool
}

type Artwork interface {
	StoreArtwork() *store.Artwork
	MessageSends(footer string, tags bool) ([]*discordgo.MessageSend, error)
	URL() string
	Len() int
}
