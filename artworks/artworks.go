package artworks

import (
	"fmt"
	"strings"
	"time"

	"mvdan.cc/xurls/v2"

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
	Render() (Rendered, error)
	ID() string
	URL() string
	Len() int
}

// RenderedImage is one page of artwork. A non-empty Original gains an
// "Original quality" field on its page when rendered.
type RenderedImage struct {
	Preview  string
	Original string
}

// RenderedField is one stat line on an embed.
type RenderedField struct {
	Name   string
	Value  string
	Inline bool
}

// Rendered is the data a Provider hands to the render module.
type Rendered struct {
	Title           string
	URL             string
	Timestamp       time.Time
	Images          []RenderedImage
	Description     string
	Tags            []string
	TagLinkTemplate string
	Fields          []RenderedField
	Files           []*discordgo.File
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
