package repost

import (
	"context"
	"fmt"
	"time"

	"github.com/ReneKroon/ttlcache"
)

type inMemory struct {
	cache *ttlcache.Cache
}

func NewMemory() Detector {
	return &inMemory{cache: ttlcache.NewCache()}
}

func (rd inMemory) Delete(_ context.Context, channelID, artworkID string) error {
	ok := rd.cache.Remove(fmt.Sprintf("%v:%v", channelID, artworkID))
	if !ok {
		return ErrNotFound
	}

	return nil
}

func (rd inMemory) Create(_ context.Context, rep *Repost, ttl time.Duration) error {
	rep.ExpiresAt = time.Now().Add(ttl)
	rd.cache.SetWithTTL(rd.key(rep), rep, ttl)

	return nil
}

func (rd inMemory) Find(_ context.Context, channelID, artworkID string) (*Repost, error) {
	rep, ok := rd.cache.Get(fmt.Sprintf("%v:%v", channelID, artworkID))
	if !ok {
		return nil, ErrNotFound
	}

	return rep.(*Repost), nil
}

func (inMemory) key(rep *Repost) string {
	return fmt.Sprintf("%v:%v", rep.ChannelID, rep.ID)
}

func (rd inMemory) Close() error {
	rd.cache.Close()

	return nil
}
