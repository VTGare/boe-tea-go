package cache

import (
	"fmt"
	"sync"
	"time"

	"github.com/ReneKroon/ttlcache"
)

// Cache represents a thread-safe map
type Cache struct {
	mx    sync.RWMutex
	cache map[string]any
}

func New() *Cache {
	return &Cache{
		cache: make(map[string]any),
	}
}

func (c *Cache) Get(key string) (any, bool) {
	c.mx.RLock()
	defer c.mx.RUnlock()

	v, ok := c.cache[key]
	return v, ok
}

func (c *Cache) MustGet(key string) any {
	c.mx.RLock()
	defer c.mx.RUnlock()

	v, ok := c.cache[key]
	if ok {
		return v
	}

	return nil
}

func (c *Cache) Set(key string, value any) {
	c.mx.Lock()
	defer c.mx.Unlock()

	c.cache[key] = value
}

func (c *Cache) Delete(key string) {
	c.mx.Lock()
	defer c.mx.Unlock()

	delete(c.cache, key)
}

func (c *Cache) Len() int {
	c.mx.Lock()
	defer c.mx.Unlock()

	return len(c.cache)
}

type EmbedCache struct {
	cache *ttlcache.Cache
}

// MessageInfo is a message/channel ID pair.
type MessageInfo struct {
	MessageID string
	ChannelID string
	ArtworkID string
}

// CachedEmbed stores information about an embed that's later retrieved in
// OnReactionAdd event to let original poster remove the embed or the entire
// gallery posted by Boe Tea if reaction was added on their original message.
// Children array is filled for parent messages only and it contains
// all embeds sent by Boe Tea by posting the message, including crossposted messages.
type CachedPost struct {
	AuthorID string
	IsParent bool
	Children []*MessageInfo
}

func (ec *EmbedCache) makeKey(channelID, messageID string) string {
	return fmt.Sprintf(
		"channel:%v:message:%v",
		channelID,
		messageID,
	)
}

func (ec *EmbedCache) Get(channelID, messageID string) (*CachedPost, bool) {
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

func (ec *EmbedCache) Set(userID, channelID, messageID string, isParent bool, children ...*MessageInfo) {
	key := ec.makeKey(
		channelID, messageID,
	)

	ec.cache.Set(key, &CachedPost{
		AuthorID: userID,
		IsParent: isParent,
		Children: children,
	})
}

func (ec *EmbedCache) Remove(channelID, messageID string) bool {
	key := ec.makeKey(
		channelID, messageID,
	)

	return ec.cache.Remove(key)
}

// NewEmbedCache creates a new embed cache for storing IDs of embeds users posted.
func NewEmbedCache() *EmbedCache {
	cache := ttlcache.NewCache()
	cache.SetTTL(15 * time.Minute)

	return &EmbedCache{cache}
}
