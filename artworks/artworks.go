package artworks

import (
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/bwmarrin/discordgo"
)

type Provider interface {
    Match(string) (string, bool)
	Find(string) (Artwork, error)
	Enabled(*store.Guild) bool
	IsTwitter() bool
}

type Artwork interface {
	StoreArtwork(string, string, string, []string) *store.Artwork
	MessageSends(string, bool) ([]*discordgo.MessageSend, error)
	GetTitle() string
	GetAuthor() string
	GetURL() string
	GetImages() []string
	Len() int
}

type ProvBase struct { Regex *regexp.Regexp }
type TwitBase struct {}
type ArtBase struct {}

func (p *ProvBase) Match(url string) (string, bool) {
	res := p.Regex.FindStringSubmatch(url)
	if res == nil {
		return "", false
	}
	return res[1], true
}

func (t *TwitBase) Match(s string) (string, bool) {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return "", false
	}

	if u.Host != "twitter.com" && u.Host != "mobile.twitter.com" {
		return "", false
	}

	parts := strings.FieldsFunc(u.Path, func(r rune) bool {
		return r == '/'
	})

	if len(parts) < 3 {
		return "", false
	}

	parts = parts[2:]
	if parts[0] == "status" {
		parts = parts[1:]
	}

	snowflake := parts[0]
	if _, err := strconv.ParseUint(snowflake, 10, 64); err != nil {
		return "", false
	}

	return snowflake, true
}

//StoreArtwork transforms an artwork to an insertable to database artwork model.
func (a *ArtBase) StoreArtwork(title string, author string, url string, images []string) *store.Artwork {
    return &store.Artwork{
        Title:  title,
        Author: author,
        URL:    url,
        Images: images,
    }
}

func (p *ProvBase) IsTwitter() bool { return false }
func (t *TwitBase) IsTwitter() bool { return true }
func (a *ArtBase) GetTitle() string { return "" }
