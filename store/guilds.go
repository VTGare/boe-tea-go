package store

import (
	"context"
	"time"
)

type GuildStore interface {
	Guild(ctx context.Context, guildID string) (*Guild, error)
	CreateGuild(ctx context.Context, guildID string) (*Guild, error)
	UpdateGuild(ctx context.Context, guildID string, field string, value any) (*Guild, error)
	AddArtChannels(ctx context.Context, guildID string, channels []string) (*Guild, error)
	DeleteArtChannels(ctx context.Context, guildID string, channels []string) (*Guild, error)
}

type Guild struct {
	ID     string `json:"id" bson:"guild_id" validate:"required"`
	Prefix string `json:"prefix" bson:"prefix" validate:"required,max=5"`

	Pixiv     bool `json:"pixiv" bson:"pixiv"`           // Deprecated: use guild.Providers instead
	Twitter   bool `json:"twitter" bson:"twitter"`       // Deprecated: use guild.Providers instead
	Deviant   bool `json:"deviant" bson:"deviant"`       // Deprecated: use guild.Providers instead
	Bluesky   bool `json:"bluesky" bson:"bluesky"`       // Deprecated: use guild.Providers instead
	SkipFirst bool `json:"skip_first" bson:"skip_first"` // Deprecated: use guild.Providers instead

	Providers map[string]*Provider `json:"providers" bson:"providers"`

	Tags       bool `json:"tags" bson:"tags"`
	FlavorText bool `json:"flavour_text" bson:"flavour_text"`
	Crosspost  bool `json:"crosspost" bson:"crosspost"`
	Reactions  bool `json:"reactions" bson:"reactions"`
	Limit      int  `json:"limit" bson:"limit" validate:"required"`

	Repost           GuildRepost   `json:"repost" bson:"repost" validate:"required"`
	RepostExpiration time.Duration `json:"repost_expiration" bson:"repost_expiration"`

	ArtChannels []string `json:"art_channels" bson:"art_channels" validate:"required"`
	NSFW        bool     `json:"nsfw" bson:"nsfw"`

	CreatedAt time.Time `json:"created_at" bson:"created_at" validate:"required"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
}

type Provider struct {
	Disabled  bool `json:"disabled" bson:"disabled"`
	SkipFirst bool `json:"skip_first" bson:"skip_first"`
	OnlyVideo bool `json:"only_video" bson:"only_video"`
}

type GuildRepost string

const (
	GuildRepostEnabled  GuildRepost = "enabled"
	GuildRepostDisabled GuildRepost = "disabled"
	GuildRepostStrict   GuildRepost = "strict"
)

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
