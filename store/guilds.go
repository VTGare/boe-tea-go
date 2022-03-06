package store

import (
	"context"
	"time"
)

type GuildStore interface {
	Guild(ctx context.Context, guildID string) (*Guild, error)
	CreateGuild(ctx context.Context, guildID string) (*Guild, error)
	UpdateGuild(ctx context.Context, guild *Guild) (*Guild, error)
	AddArtChannels(ctx context.Context, guildID string, channels []string) (*Guild, error)
	DeleteArtChannels(ctx context.Context, guildID string, channels []string) (*Guild, error)
}

type Guild struct {
	ID     string `json:"id" bson:"guild_id" validate:"required"`
	Prefix string `json:"prefix" bson:"prefix" validate:"required,max=5"`

	Pixiv      bool `json:"pixiv" bson:"pixiv"`
	Twitter    bool `json:"twitter" bson:"twitter"`
	Deviant    bool `json:"deviant" bson:"deviant"`
	Artstation bool `json:"artstation" bson:"artstation"`

	Tags      bool `json:"tags" bson:"tags"`
	Footer    bool `json:"footer" bson:"footer"`
	Crosspost bool `json:"crosspost" bson:"crosspost"`
	Reactions bool `json:"reactions" bson:"reactions"`
	Limit     int  `json:"limit" bson:"limit" validate:"required"`

	Repost           string        `json:"repost" bson:"repost" validate:"required"`
	RepostExpiration time.Duration `json:"repost_expiration" bson:"repost_expiration"`

	ArtChannels []string `json:"art_channels" bson:"art_channels" validate:"required"`
	NSFW        bool     `json:"nsfw" bson:"nsfw"`

	CreatedAt time.Time `json:"created_at" bson:"created_at" validate:"required"`
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`
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
		Tags:             true,
		Footer:           true,
		Repost:           "enabled",
		RepostExpiration: 24 * time.Hour,
		Crosspost:        true,
		Reactions:        true,
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
		Tags:             true,
		Footer:           true,
		Repost:           "disabled",
		RepostExpiration: 24 * time.Hour,
		Crosspost:        true,
		Reactions:        true,
	}
}
