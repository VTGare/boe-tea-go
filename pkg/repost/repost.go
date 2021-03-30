package repost

import "time"

type Detector interface {
	Find(channelID string, artworkID string) (*Repost, error)
	Create(*Repost, time.Duration) error
}

type Repost struct {
	ID        string `redis:"id"`
	URL       string `redis:"url"`
	GuildID   string `redis:"guild_id"`
	ChannelID string `redis:"channel_id"`
	MessageID string `redis:"message_id"`
	Expire    time.Time
}
