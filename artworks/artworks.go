package artworks

import (
	"fmt"
	"regexp"
	"strings"

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
	ID() string
	URL() string
	Len() int
}

func NewError(p Provider, err error) error {
	return &Error{
		provider: fmt.Sprintf("%T", p),
		cause:    err,
	}
}

func EscapeMarkdown(content string) string {
	contents := strings.Split(content, "\n")
	regex := regexp.MustCompile("^#{1,3}")

	for i, line := range contents {
		if regex.MatchString(line) {
			contents[i] = "\\" + line
		}
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
