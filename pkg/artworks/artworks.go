package artworks

import (
	"github.com/VTGare/boe-tea-go/pkg/models/artworks"
	"github.com/bwmarrin/discordgo"
)

type Provider interface {
	Match(string) (string, bool)
	Find(string) (Artwork, error)
}

type Artwork interface {
	ToModel() *artworks.ArtworkInsert
	Embeds(string) []*discordgo.MessageEmbed
	URL() string
}
