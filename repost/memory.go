package repost

import (
	"fmt"
	"time"

	cache "github.com/patrickmn/go-cache"
)

type inMemory struct {
	cache *cache.Cache
}

func NewMemory() Detector {
	return &inMemory{cache: cache.New(0, 5*time.Minute)}
}

func (rd inMemory) Create(rep *Repost, ttl time.Duration) error {
	rep.ExpiresAt = time.Now().Add(ttl)
	rd.cache.Set(rd.key(rep), rep, ttl)

	return nil
}

func (rd inMemory) Find(channelID, artworkID string) (*Repost, error) {
	rep, ok := rd.cache.Get(fmt.Sprintf("%v:%v", channelID, artworkID))
	if !ok {
		return nil, ErrNotFound
	}

	return rep.(*Repost), nil
}

func (rd inMemory) key(rep *Repost) string {
	return fmt.Sprintf("%v:%v", rep.ChannelID, rep.ID)
}

func (rd inMemory) Close() error {
	rd.cache.Flush()
	return nil
}
