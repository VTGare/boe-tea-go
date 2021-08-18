package artworks

import (
	"github.com/VTGare/boe-tea-go/pkg/models/artworks"
	"github.com/VTGare/boe-tea-go/pkg/models/guilds"
	"github.com/bwmarrin/discordgo"
)

type Provider interface {
	Match(url string) (string, bool)
	Find(id string) (Artwork, error)
	Enabled(*guilds.Guild) bool
}

type Artwork interface {
	ToModel() *artworks.ArtworkInsert
	MessageSends(footer string) ([]*discordgo.MessageSend, error)
	URL() string
	Len() int
}
