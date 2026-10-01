package store

import (
	"context"
	"errors"
	"slices"
	"time"
)

type GuildStore interface {
	// Guild returns the guild. A miss reports ErrGuildNotFound. An
	// empty ID returns the DM guild and never misses.
	Guild(ctx context.Context, guildID string) (*Guild, error)

	// CreateGuild creates the guild with defaults.
	CreateGuild(ctx context.Context, guildID string) (*Guild, error)

	// UpdateGuild replaces the guild. A missing guild reports
	// ErrGuildNotFound.
	UpdateGuild(ctx context.Context, guild *Guild) (*Guild, error)

	// AddArtChannels adds channels, ignoring ones already present. A
	// missing guild reports ErrGuildNotFound.
	AddArtChannels(ctx context.Context, guildID string, channels []string) (*Guild, error)

	// DeleteArtChannels removes channels, ignoring ones not present. A
	// missing guild reports ErrGuildNotFound.
	DeleteArtChannels(ctx context.Context, guildID string, channels []string) (*Guild, error)
}

// Guild is one server's settings.
type Guild struct {
	ID     string `json:"id" bson:"guild_id"`
	Prefix string `json:"prefix" bson:"prefix"`

	Posting Posting `json:"posting" bson:"posting"`
	Repost  Repost  `json:"repost" bson:"repost"`

	// DisabledProviders lists the keys (artworks.Info.Key) of turned-off
	// artwork sources. Everything else is on.
	DisabledProviders []string `json:"disabled_providers" bson:"disabled_providers"`

	// ArtChannels restricts posting to these channels. Empty means all.
	ArtChannels []string `json:"art_channels" bson:"art_channels"`

	CreatedAt time.Time `json:"created_at" bson:"created_at"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

// Posting controls how artwork posts look.
type Posting struct {
	// Limit caps the images per post.
	Limit int `json:"limit" bson:"limit"`

	Tags      bool `json:"tags" bson:"tags"`
	Reactions bool `json:"reactions" bson:"reactions"`
	Crosspost bool `json:"crosspost" bson:"crosspost"`

	// SkipFirstTweet drops a tweet's first image, which Discord already
	// previews.
	SkipFirstTweet bool `json:"skip_first_tweet" bson:"skip_first_tweet"`

	// Quotes adds a random quote to post footers; NSFWQuotes allows NSFW
	// ones among them.
	Quotes     bool `json:"quotes" bson:"quotes"`
	NSFWQuotes bool `json:"nsfw_quotes" bson:"nsfw_quotes"`
}

// Repost controls repost detection.
type Repost struct {
	Mode RepostMode `json:"mode" bson:"mode"`

	// TTL is how long a posted link counts towards reposts.
	TTL time.Duration `json:"ttl" bson:"ttl"`
}

type RepostMode string

const (
	RepostOff RepostMode = "off"
	// RepostNotify replies with a notice when a link was already posted.
	RepostNotify RepostMode = "notify"
	// RepostStrict also deletes the repost.
	RepostStrict RepostMode = "strict"
)

// Setting limits. Keep them in sync with the CHECK constraints in the
// Postgres migrations.
const (
	MaxPrefixLength = 5
	MinPostLimit    = 1
	MaxPostLimit    = 100
	MinRepostTTL    = time.Minute
	MaxRepostTTL    = 7 * 24 * time.Hour
)

// ProviderEnabled reports whether links from the provider with key are
// handled.
func (g *Guild) ProviderEnabled(key string) bool {
	return g != nil && !slices.Contains(g.DisabledProviders, key)
}

// SetProvider turns the provider with key on or off.
func (g *Guild) SetProvider(key string, enabled bool) {
	g.DisabledProviders = slices.DeleteFunc(g.DisabledProviders, func(d string) bool { return d == key })
	if !enabled {
		g.DisabledProviders = append(g.DisabledProviders, key)
	}
}

// PostsIn reports whether artwork is posted in the channel. With no art
// channels set, every channel counts.
func (g *Guild) PostsIn(channelID string) bool {
	return len(g.ArtChannels) == 0 || slices.Contains(g.ArtChannels, channelID)
}

// GetOrCreateGuild returns the guild, creating it with defaults on
// ErrGuildNotFound.
func GetOrCreateGuild(ctx context.Context, s Store, guildID string) (*Guild, bool, error) {
	guild, err := s.Guild(ctx, guildID)
	if err == nil {
		return guild, false, nil
	}

	if !errors.Is(err, ErrGuildNotFound) {
		return nil, false, err
	}

	guild, err = s.CreateGuild(ctx, guildID)
	if err != nil {
		return nil, false, err
	}

	return guild, true, nil
}

func DefaultGuild(id string) *Guild {
	now := time.Now()

	return &Guild{
		ID:     id,
		Prefix: "bt!",
		Posting: Posting{
			Limit:          10,
			Tags:           true,
			Crosspost:      true,
			SkipFirstTweet: true,
			Quotes:         true,
			NSFWQuotes:     true,
		},
		Repost:            Repost{Mode: RepostNotify, TTL: 24 * time.Hour},
		DisabledProviders: make([]string, 0),
		ArtChannels:       make([]string, 0),
		CreatedAt:         now,
		UpdatedAt:         now,
	}
}

// UserGuild is the settings used in DMs.
func UserGuild() *Guild {
	return &Guild{
		Prefix: "bt!",
		Posting: Posting{
			Limit:          100,
			Tags:           true,
			Reactions:      true,
			SkipFirstTweet: true,
			Quotes:         true,
			NSFWQuotes:     true,
		},
		Repost:            Repost{Mode: RepostOff},
		DisabledProviders: make([]string, 0),
		ArtChannels:       make([]string, 0),
	}
}
