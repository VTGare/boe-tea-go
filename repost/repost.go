package repost

import (
	"context"
	"errors"
	"time"
)

var ErrNotFound = errors.New("repost not found")

type Detector interface {
	Find(ctx context.Context, channelID string, artworkID string) (*Repost, error)
	Create(ctx context.Context, repost *Repost, duration time.Duration) error
	Delete(ctx context.Context, channelID string, artworkID string) error
	Close() error
}

type Repost struct {
	ID        string `redis:"id"`
	URL       string `redis:"url"`
	GuildID   string `redis:"guild_id"`
	ChannelID string `redis:"channel_id"`
	MessageID string `redis:"message_id"`
	ExpiresAt time.Time
}
