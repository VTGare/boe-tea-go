package artworks

import (
	"errors"
	"fmt"
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
	URL() string
	Len() int
}

type Error struct {
	provider string
	cause    error
}

func (e *Error) Error() string {
	return fmt.Sprintf("provider %v returned an error: %v", e.provider, e.cause.Error())
}

func (e *Error) Unwrap() error {
	return e.cause
}

func NewError(p Provider, err error) error {
	return &Error{
		provider: fmt.Sprintf("%T", p),
		cause:    err,
	}
}

// Common errors
var (
	ErrArtworkNotFound = errors.New("artwork not found")
	ErrRateLimited     = errors.New("provider rate limited")
)

func IsAIGenerated(contents ...string) bool {
	aiTags := []string{
		"aiart",
		"aigenerated",
		"aiイラスト",
		"createdwithai",
		"dall-e",
		"midjourney",
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
