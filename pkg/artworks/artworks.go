package artworks

import (
	"github.com/VTGare/boe-tea-go/pkg/models/artworks"
	"github.com/VTGare/boe-tea-go/pkg/models/guilds"
	"github.com/bwmarrin/discordgo"
)

type Provider interface {
	Match(string) (string, bool)
	Find(string) (Artwork, error)
	Enabled(*guilds.Guild) bool
}

type Artwork interface {
	ToModel() *artworks.ArtworkInsert
	Embeds(string) []*discordgo.MessageEmbed
	URL() string
}
