package bluesky

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"
	"github.com/bwmarrin/discordgo"
	"github.com/julien040/go-ternary"
)

type Bluesky struct {
	regex  *regexp.Regexp
	client *http.Client
}

type Response struct {
	Thread struct {
		Post *Post `json:"post"`
	} `json:"thread"`
}

type Post struct {
	URI    string `json:"uri,omitempty"`
	Author struct {
		DID         string    `json:"did,omitempty"`
		Handle      string    `json:"handle,omitempty"`
		DisplayName string    `json:"displayName,omitempty"`
		Avatar      string    `json:"avatar,omitempty"`
		CreatedAt   time.Time `json:"createdAt,omitempty"`
	} `json:"author,omitempty"`
	Embed struct {
		Type      EmbedType `json:"$type,omitempty"`
		Playlist  string    `json:"playlist,omitempty"`  // For videos
		Thumbnail string    `json:"thumbnail,omitempty"` // For videos
		Images    []struct {
			Thumb    string `json:"thumb,omitempty"`
			Fullsize string `json:"fullsize,omitempty"`
		} `json:"images,omitempty"`
	} `json:"embed,omitempty"`
	Record struct {
		Facets []struct {
			Features []struct {
				Type string `json:"$type,omitempty"`
				Tag  string `json:"tag,omitempty"`
			} `json:"features,omitempty"`
		} `json:"facets,omitempty"`
		Text      string    `json:"text,omitempty"`
		CreatedAt time.Time `json:"createdAt,omitempty"`
	} `json:"record,omitempty"`

	ReplyCount  int       `json:"replyCount,omitempty"`
	RepostCount int       `json:"repostCount,omitempty"`
	LikeCount   int       `json:"likeCount,omitempty"`
	IndexedAt   time.Time `json:"indexedAt,omitempty"`
}

type EmbedType string

const (
	EmbedTypeVideo EmbedType = "app.bsky.embed.video#view"
	EmbedTypeImage EmbedType = "app.bsky.embed.images#view"
)

type Artwork struct {
	id  string
	url string

	AuthorHandle      string
	AuthorDisplayName string
	Text              string
	Tags              []string
	Images            []string

	Likes       int
	Reposts     int
	Replies     int
	AIGenerated bool
	CreatedAt   time.Time
}

func New() *Bluesky {
	return &Bluesky{
		regex:  regexp.MustCompile(`(?i)https://(?:www\.)?bsky\.app/profile/(\w.+)/post/([\w\-]+)`),
		client: http.DefaultClient,
	}
}

// Enabled implements artworks.Provider.
func (*Bluesky) Enabled(g *store.Guild) bool {
	return g.Bluesky
}

// Find implements artworks.Provider.
func (b *Bluesky) Find(id string) (artworks.Artwork, error) {
	return artworks.WrapError(b, func() (artworks.Artwork, error) {
		did, key, _ := strings.Cut(id, ":")
		atURI := fmt.Sprintf("at://%v/app.bsky.feed.post/%v", did, key)

		resp, err := b.client.Get("https://public.api.bsky.app/xrpc/app.bsky.feed.getPostThread?uri=" + atURI + "&depth=0")
		if err != nil {
			return nil, fmt.Errorf("http get: %w", err)
		}
		defer resp.Body.Close()

		switch resp.StatusCode {
		case http.StatusOK:
			break

		case http.StatusBadRequest:
			fallthrough
		case http.StatusInternalServerError:
			return nil, artworks.ErrArtworkNotFound
		default:
			return nil, fmt.Errorf("unexpected response status: %v", resp.Status)
		}

		decoded := &Response{}
		if err := json.NewDecoder(resp.Body).Decode(decoded); err != nil {
			return nil, fmt.Errorf("decode: %w", err)
		}

		tags := make([]string, 0)
		for _, facet := range decoded.Thread.Post.Record.Facets {
			for _, feature := range facet.Features {
				if feature.Type != "app.bsky.richtext.facet#tag" {
					continue
				}

				tags = append(tags, feature.Tag)
			}
		}

		var images []string
		switch decoded.Thread.Post.Embed.Type {
		case EmbedTypeVideo:
			images = []string{decoded.Thread.Post.Embed.Thumbnail}
		case EmbedTypeImage:
			images = make([]string, 0, len(decoded.Thread.Post.Embed.Images))
			for _, image := range decoded.Thread.Post.Embed.Images {
				images = append(images, image.Fullsize)
			}
		}

		return &Artwork{
			id:  id,
			url: fmt.Sprintf("https://bsky.app/profile/%v/post/%v", did, key),

			AuthorHandle:      decoded.Thread.Post.Author.Handle,
			AuthorDisplayName: decoded.Thread.Post.Author.DisplayName,

			Tags:   tags,
			Images: images,

			Text:        decoded.Thread.Post.Record.Text,
			Likes:       decoded.Thread.Post.LikeCount,
			Reposts:     decoded.Thread.Post.RepostCount,
			Replies:     decoded.Thread.Post.ReplyCount,
			CreatedAt:   decoded.Thread.Post.Record.CreatedAt,
			AIGenerated: false,
		}, nil
	})
}

// Match implements artworks.Provider.
func (b *Bluesky) Match(url string) (string, bool) {
	res := b.regex.FindStringSubmatch(url)
	if res == nil {
		return "", false
	}

	return res[1] + ":" + res[2], true
}

// MessageSends implements artworks.Artwork.
func (a *Artwork) MessageSends(footer string, tagsEnabled bool) ([]*discordgo.MessageSend, error) {
	eb := embeds.NewBuilder()
	eb.URL(a.url).Timestamp(a.CreatedAt)

	if a.Reposts > 0 {
		eb.AddField("Reposts", strconv.Itoa(a.Reposts), true)
	}

	if a.Likes > 0 {
		eb.AddField("Likes", strconv.Itoa(a.Likes), true)
	}

	if footer != "" {
		eb.Footer(footer, "")
	}

	if a.AIGenerated {
		eb.AddField("⚠️ Disclaimer", "This artwork is AI-generated.")
	}

	length := len(a.Images)
	posts := make([]*discordgo.MessageSend, 0, length)
	eb.Title(ternary.If(length > 1,
		fmt.Sprintf("%v (%v) | Page %v / %v", a.AuthorDisplayName, a.AuthorHandle, 1, length),
		fmt.Sprintf("%v (%v)", a.AuthorDisplayName, a.AuthorHandle),
	))

	if length > 0 {
		eb.Image(a.Images[0])
	}

	desc := a.Text
	if tagsEnabled && len(a.Tags) > 0 {
		desc = fmt.Sprintf("%v\n\n**Tags**\n%v", desc, strings.Join(a.Tags, " • "))
	}

	eb.Description(desc)

	posts = append(posts, &discordgo.MessageSend{
		Embeds: []*discordgo.MessageEmbed{eb.Finalize()},
	})

	if len(a.Images) > 1 {
		for ind, photo := range a.Images[1:] {
			eb := embeds.NewBuilder()

			eb.Title(fmt.Sprintf("%v (%v) | Page %v / %v", a.AuthorDisplayName, a.AuthorHandle, ind+2, length)).URL(a.url)
			eb.Image(photo).Timestamp(a.CreatedAt)

			if footer != "" {
				eb.Footer(footer, "")
			}

			posts = append(posts, &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}})
		}
	}

	return posts, nil
}

// ID implements artworks.Artwork.
func (a *Artwork) ID() string {
	return a.id
}

// Len implements artworks.Artwork.
func (a *Artwork) Len() int {
	return len(a.Images)
}

// StoreArtwork implements artworks.Artwork.
func (a *Artwork) StoreArtwork() *store.Artwork {
	return &store.Artwork{
		Author: a.AuthorHandle,
		Images: a.Images,
		URL:    a.url,
	}
}

// URL implements artworks.Artwork.
func (a *Artwork) URL() string {
	return a.url
}
