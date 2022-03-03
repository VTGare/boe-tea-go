package repost

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound = errors.New("repost not found")
)

type Detector interface {
	Find(channelID string, artworkID string) (*Repost, error)
	Create(*Repost, time.Duration) error
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

func (r *Repost) String() string {
	return fmt.Sprintf("https://discord.com/channels/%v/%v/%v", r.GuildID, r.ChannelID, r.MessageID)
}
