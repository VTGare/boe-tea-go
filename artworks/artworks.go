package artworks

import (
	"fmt"
	"strings"
	"time"

	"mvdan.cc/xurls/v2"

	"github.com/VTGare/boe-tea-go/store"
	"github.com/disgoorg/disgo/discord"
)

type Provider interface {
	Info() Info
	Match(url string) (string, bool)
	Find(id string) (Artwork, error)
}

// Info describes a provider to guild settings.
type Info struct {
	// Key is saved in guild settings and is also the provider's /set
	// subcommand, so stick to lowercase letters, digits and dashes, and
	// never change it after release.
	Key string
	// Label is the display name in embeds.
	Label   string
	Aliases []string
}

// SourceKey identifies an artwork on its provider, e.g. "twitter:123".
func SourceKey(p Provider, url string) (string, bool) {
	id, ok := p.Match(url)
	if !ok {
		return "", false
	}

	return p.Info().Key + ":" + id, true
}

type Artwork interface {
	StoreArtwork() *store.Artwork
	Render() (Rendered, error)
	ID() string
	URL() string
	Len() int
}

// RenderedImage is a single page. If Original is set, the page gets an
// "Original quality" link.
type RenderedImage struct {
	Preview  string
	Original string
}

// RenderedField is an extra name/value line on the embed.
type RenderedField struct {
	Name   string
	Value  string
	Inline bool
}

// Rendered is everything the render package needs to build a provider's embeds.
type Rendered struct {
	Title           string
	URL             string
	Timestamp       time.Time
	Images          []RenderedImage
	Description     string
	Tags            []string
	TagLinkTemplate string
	Fields          []RenderedField
	Files           []*discord.File
	AIGenerated     bool
}

func EscapeMarkdown(content string) string {
	replaced := xurls.Strict().ReplaceAllString(content, "%s")
	contents := strings.Split(replaced, "\n")

	for i, line := range contents {
		newLine := line
		for _, ch := range ".-_|#~<>*" {
			str := string(ch)
			newLine = strings.ReplaceAll(newLine, str, "\\"+str)
		}
		contents[i] = newLine
	}

	urls := xurls.Strict().FindAllString(content, -1)
	var anyUrls []any
	for _, url := range urls {
		anyUrls = append(anyUrls, url)
	}

	return fmt.Sprintf(strings.Join(contents, "\n"), anyUrls...)
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
