package artworks

import (
	"strings"

	"github.com/VTGare/boe-tea-go/store"
	"github.com/bwmarrin/discordgo"
)

type Provider interface {
	Name() string // Name gets a name of the provider for referencing in settings and the database or identifying it without using reflection
	Match(url string) (string, bool)
	Find(id string) (Artwork, error)
	Enabled(*store.Guild) bool
}

type ProviderBase struct {
	name string
}

func (pb *ProviderBase) Name() string {
	return pb.name
}

func (pb *ProviderBase) Enabled(g *store.Guild) bool {
	provider, ok := g.Providers[pb.name]
	if !ok {
		// All providers are enabled by default, if provider entry doesn't exist then
		// it wasn't created in the database yet because no one attempted to change it.
		// Therefore, we return true.
		return true
	}

	return !provider.Disabled
}

func NewProviderBase(name string) ProviderBase {
	return ProviderBase{name: name}
}

type Artwork interface {
	StoreArtwork() *store.Artwork
	MessageSends(footer string, tags bool) ([]*discordgo.MessageSend, error)
	ID() string
	URL() string
	Len() int
}

func EscapeMarkdown(content string) string {
	contents := strings.Split(content, "\n")
	escape := []string{
		".", "-", "_", "|", "#",
		"~", "<", ">", "*",
	}

	for i, line := range contents {
		newLine := line
		for _, s := range escape {
			newLine = strings.ReplaceAll(newLine, s, "\\"+s)
		}
		contents[i] = newLine
	}
	return strings.Join(contents, "\n")
}

func IsAIGenerated(contents ...string) bool {
	aiTags := []string{
		"aiart",
		"aigenerated",
		"aiイラスト",
		"createdwithai",
		"dall-e",
		"midjourney",
		"nijijourney",
		"stablediffusion",
	}

	for _, tag := range contents {
		for _, test := range aiTags {
			if strings.EqualFold(tag, test) {
				return true
			}
		}
	}
	return false
}
