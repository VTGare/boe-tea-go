package cache

import (
	"fmt"
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
	cache "github.com/patrickmn/go-cache"
)

type EmbedCache struct {
	cache *cache.Cache
}

//MessageInfo is a message/channel ID pair.
type MessageInfo struct {
	MessageID discord.MessageID
	ChannelID discord.ChannelID
}

// CachedEmbed stores information about an embed that's later retrieved in
// OnReactionAdd event to let original poster remove the embed or the entire
// gallery posted by Boe Tea if reaction was added on their original message.
// Children array is filled for parent messages only and it contains
// all embeds sent by Boe Tea by posting the message, including crossposted messages.
type CachedPost struct {
	AuthorID discord.UserID
	Parent   bool
	Children []*MessageInfo
}

func (ec *EmbedCache) makeKey(channelID discord.ChannelID, messageID discord.MessageID) string {
	return fmt.Sprintf(
		"channel:%v:message:%v",
		channelID.String(),
		messageID.String(),
	)
}

func (ec *EmbedCache) Get(channelID discord.ChannelID, messageID discord.MessageID) (*CachedPost, bool) {
	key := ec.makeKey(
		channelID, messageID,
	)

	if embed, ok := ec.cache.Get(key); ok {
		if embed, ok := embed.(*CachedPost); ok {
			return embed, true
		}
	}

	return nil, false
}

func (ec *EmbedCache) Set(userID discord.UserID, channelID discord.ChannelID, messageID discord.MessageID, parent bool, children ...*MessageInfo) {
	key := ec.makeKey(channelID, messageID)

	ec.cache.Set(key, &CachedPost{
		AuthorID: userID,
		Parent:   parent,
		Children: children,
	}, 0)
}

func (ec *EmbedCache) Delete(channelID discord.ChannelID, messageID discord.MessageID) {
	key := ec.makeKey(channelID, messageID)
	ec.cache.Delete(key)
}

//NewEmbedCache creates a new embed cache for storing IDs of embeds users posted.
func NewEmbedCache() *EmbedCache {
	cache := cache.New(15*time.Minute, 20*time.Minute)
	return &EmbedCache{cache}
}
