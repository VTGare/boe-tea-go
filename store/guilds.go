package store

import (
	"context"
	"errors"
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

type Guild struct {
	ID     string `json:"id" bson:"guild_id" validate:"required"`
	Prefix string `json:"prefix" bson:"prefix" validate:"required,max=5"`

	Pixiv   bool `json:"pixiv" bson:"pixiv"`
	Twitter bool `json:"twitter" bson:"twitter"`
	Deviant bool `json:"deviant" bson:"deviant"`
	Bluesky bool `json:"bluesky" bson:"bluesky"`

	Tags       bool `json:"tags" bson:"tags"`
	FlavorText bool `json:"flavour_text" bson:"flavour_text"`
	Crosspost  bool `json:"crosspost" bson:"crosspost"`
	Reactions  bool `json:"reactions" bson:"reactions"`
	SkipFirst  bool `json:"skip_first" bson:"skip_first"`
	Limit      int  `json:"limit" bson:"limit" validate:"required"`

	Repost           GuildRepost   `json:"repost" bson:"repost" validate:"required"`
	RepostExpiration time.Duration `json:"repost_expiration" bson:"repost_expiration"`

	ArtChannels []string `json:"art_channels" bson:"art_channels" validate:"required"`
	NSFW        bool     `json:"nsfw" bson:"nsfw"`

	CreatedAt time.Time `json:"created_at" bson:"created_at" validate:"required"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

type GuildRepost string

const (
	GuildRepostEnabled  GuildRepost = "enabled"
	GuildRepostDisabled GuildRepost = "disabled"
	GuildRepostStrict   GuildRepost = "strict"
)

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
	return &Guild{
		ID:               id,
		Prefix:           "bt!",
		Limit:            10,
		NSFW:             true,
		Pixiv:            true,
		Twitter:          true,
		Deviant:          true,
		Bluesky:          true,
		Tags:             true,
		FlavorText:       true,
		Repost:           GuildRepostEnabled,
		RepostExpiration: 24 * time.Hour,
		Crosspost:        true,
		Reactions:        false,
		SkipFirst:        false,
		ArtChannels:      make([]string, 0),
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
}

func UserGuild() *Guild {
	return &Guild{
		ID:               "",
		Prefix:           "bt!",
		Limit:            100,
		NSFW:             true,
		Pixiv:            true,
		Twitter:          true,
		Deviant:          true,
		Bluesky:          true,
		Tags:             true,
		FlavorText:       true,
		SkipFirst:        true,
		Repost:           GuildRepostDisabled,
		RepostExpiration: 0,
		Crosspost:        false,
		Reactions:        true,
	}
}
