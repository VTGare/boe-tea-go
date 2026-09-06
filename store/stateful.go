package store

import (
	"context"
	"sort"
	"strconv"

	cache "github.com/patrickmn/go-cache"
)

type StatefulStore struct {
	Store
	cache *cache.Cache
}

func NewStatefulStore(store Store, c *cache.Cache) Store {
	return &StatefulStore{
		Store: store,
		cache: c,
	}
}

func guildKey(guildID string) string {
	return "guilds:" + guildID
}

func artworkKey(id int) string {
	return "artworks:" + strconv.Itoa(id)
}

func cloneGuild(g *Guild) *Guild {
	if g == nil {
		return nil
	}

	c := *g

	if g.ArtChannels != nil {
		c.ArtChannels = append([]string{}, g.ArtChannels...)
	}

	return &c
}

func cloneArtwork(a *Artwork) *Artwork {
	if a == nil {
		return nil
	}

	c := *a

	if a.Images != nil {
		c.Images = append([]string{}, a.Images...)
	}

	return &c
}

func cacheGet[T *Guild | *Artwork](c *cache.Cache, key string, clone func(T) T) (T, bool) {
	if cached, ok := c.Get(key); ok {
		if v, ok := cached.(T); ok && v != nil {
			return clone(v), true
		}

		c.Delete(key)
	}

	var zero T

	return zero, false
}

func cacheSet[T *Guild | *Artwork](c *cache.Cache, key string, v T, clone func(T) T) {
	c.Set(key, clone(v), 0)
}

func getOrFill[T *Guild | *Artwork](c *cache.Cache, key string, clone func(T) T, fill func() (T, error)) (T, error) {
	if v, ok := cacheGet(c, key, clone); ok {
		return v, nil
	}

	v, err := fill()
	if err != nil {
		var zero T

		return zero, err
	}

	cacheSet(c, key, v, clone)

	return v, nil
}

func writeThrough[T *Guild | *Artwork](c *cache.Cache, key string, clone func(T) T, write func() (T, error)) (T, error) {
	v, err := write()
	if err != nil {
		var zero T

		return zero, err
	}

	if v != nil {
		cacheSet(c, key, v, clone)
	}

	return v, nil
}

func (s *StatefulStore) Guild(ctx context.Context, guildID string) (*Guild, error) {
	return getOrFill(s.cache, guildKey(guildID), cloneGuild, func() (*Guild, error) {
		return s.Store.Guild(ctx, guildID)
	})
}

func (s *StatefulStore) CreateGuild(ctx context.Context, guildID string) (*Guild, error) {
	return writeThrough(s.cache, guildKey(guildID), cloneGuild, func() (*Guild, error) {
		return s.Store.CreateGuild(ctx, guildID)
	})
}

func (s *StatefulStore) UpdateGuild(ctx context.Context, guild *Guild) (*Guild, error) {
	return writeThrough(s.cache, guildKey(guild.ID), cloneGuild, func() (*Guild, error) {
		return s.Store.UpdateGuild(ctx, guild)
	})
}

func (s *StatefulStore) AddArtChannels(ctx context.Context, guildID string, channels []string) (*Guild, error) {
	return writeThrough(s.cache, guildKey(guildID), cloneGuild, func() (*Guild, error) {
		return s.Store.AddArtChannels(ctx, guildID, channels)
	})
}

func (s *StatefulStore) DeleteArtChannels(ctx context.Context, guildID string, channels []string) (*Guild, error) {
	return writeThrough(s.cache, guildKey(guildID), cloneGuild, func() (*Guild, error) {
		return s.Store.DeleteArtChannels(ctx, guildID, channels)
	})
}

func (s *StatefulStore) Artwork(ctx context.Context, id int, url string) (*Artwork, error) {
	if id != 0 {
		if artwork, ok := cacheGet(s.cache, artworkKey(id), cloneArtwork); ok {
			return artwork, nil
		}
	}

	artwork, err := s.Store.Artwork(ctx, id, url)
	if err != nil {
		return nil, err
	}

	if artwork != nil {
		cacheSet(s.cache, artworkKey(artwork.ID), artwork, cloneArtwork)
	}

	return artwork, nil
}

func (s *StatefulStore) CreateArtwork(ctx context.Context, a *Artwork) (*Artwork, error) {
	artwork, err := s.Store.CreateArtwork(ctx, a)
	if err != nil {
		return nil, err
	}

	if artwork != nil {
		cacheSet(s.cache, artworkKey(artwork.ID), artwork, cloneArtwork)
	}

	return artwork, nil
}

func (s *StatefulStore) AddBookmark(ctx context.Context, fav *Bookmark) (bool, error) {
	won, err := s.Store.AddBookmark(ctx, fav)
	if err != nil {
		return won, err
	}

	if won && fav != nil {
		s.cache.Delete(artworkKey(fav.ArtworkID))
	}

	return won, nil
}

func (s *StatefulStore) DeleteBookmark(ctx context.Context, fav *Bookmark) (bool, error) {
	deleted, err := s.Store.DeleteBookmark(ctx, fav)
	if err != nil {
		return deleted, err
	}

	if deleted && fav != nil {
		s.cache.Delete(artworkKey(fav.ArtworkID))
	}

	return deleted, nil
}

func (s *StatefulStore) SearchArtworks(ctx context.Context, filter ArtworkFilter, opts ...ArtworkSearchOptions) ([]*Artwork, error) {
	if len(filter.IDs) == 0 {
		return s.Store.SearchArtworks(ctx, filter, opts...)
	}

	opt := DefaultSearchOptions()
	if len(opts) != 0 {
		opt = opts[0]
	}

	var (
		artworks = make([]*Artwork, 0, len(filter.IDs))
		missing  = make([]int, 0, len(filter.IDs))
	)

	for _, id := range filter.IDs {
		if artwork, ok := cacheGet(s.cache, artworkKey(id), cloneArtwork); ok {
			artworks = append(artworks, artwork)

			continue
		}

		missing = append(missing, id)
	}

	if len(missing) != 0 {
		query := filter
		query.IDs = missing

		fresh, err := s.Store.SearchArtworks(ctx, query, opts...)
		if err != nil {
			return nil, err
		}

		for _, artwork := range fresh {
			if artwork == nil {
				continue
			}

			cacheSet(s.cache, artworkKey(artwork.ID), artwork, cloneArtwork)
			artworks = append(artworks, artwork)
		}
	}

	switch opt.Sort {
	case ByPopularity:
		sort.Slice(artworks, func(i, j int) bool {
			if opt.Order == Ascending {
				return artworks[i].Favorites < artworks[j].Favorites
			}

			return artworks[i].Favorites > artworks[j].Favorites
		})
	case ByTime:
		sort.Slice(artworks, func(i, j int) bool {
			if opt.Order == Ascending {
				return artworks[i].CreatedAt.Before(artworks[j].CreatedAt)
			}

			return artworks[i].CreatedAt.After(artworks[j].CreatedAt)
		})
	}

	return artworks, nil
}
