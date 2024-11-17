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

func (s *StatefulStore) Guild(ctx context.Context, guildID string) (*Guild, error) {
	if g, ok := s.cache.Get("guilds:" + guildID); ok {
		guild := g.(*Guild)
		return guild, nil
	}

	guild, err := s.Store.Guild(ctx, guildID)
	if err != nil {
		return nil, err
	}

	s.cache.Set("guilds:"+guildID, guild, 0)
	return guild, nil
}

func (s *StatefulStore) CreateGuild(ctx context.Context, guildID string) (*Guild, error) {
	guild, err := s.Store.CreateGuild(ctx, guildID)
	if err != nil {
		return nil, err
	}

	s.cache.Set("guilds:"+guildID, guild, 0)
	return guild, nil
}

func (s *StatefulStore) UpdateGuild(ctx context.Context, guild *Guild) (*Guild, error) {
	guild, err := s.Store.UpdateGuild(ctx, guild)
	if err != nil {
		return nil, err
	}

	s.cache.Set("guilds:"+guild.ID, guild, 0)
	return guild, nil
}

func (s *StatefulStore) AddArtChannels(ctx context.Context, guildID string, channels []string) (*Guild, error) {
	guild, err := s.Store.AddArtChannels(ctx, guildID, channels)
	if err != nil {
		return nil, err
	}

	s.cache.Set("guilds:"+guild.ID, guild, 0)
	return guild, nil
}

func (s *StatefulStore) DeleteArtChannels(ctx context.Context, guildID string, channels []string) (*Guild, error) {
	guild, err := s.Store.DeleteArtChannels(ctx, guildID, channels)
	if err != nil {
		return nil, err
	}

	s.cache.Set("guilds:"+guild.ID, guild, 0)
	return guild, nil
}

func (s *StatefulStore) Artwork(ctx context.Context, id int, url string) (*Artwork, error) {
	if a, ok := s.cache.Get("artworks:" + strconv.Itoa(id)); ok {
		artwork := a.(*Artwork)
		return artwork, nil
	}

	artwork, err := s.Store.Artwork(ctx, id, url)
	if err != nil {
		return nil, err
	}

	s.cache.Set("artworks:"+strconv.Itoa(artwork.ID), artwork, 0)
	return artwork, nil
}

func (s *StatefulStore) CreateArtwork(ctx context.Context, a *Artwork) (*Artwork, error) {
	artwork, err := s.Store.CreateArtwork(ctx, a)
	if err != nil {
		return nil, err
	}

	s.cache.Set("artworks:"+strconv.Itoa(artwork.ID), artwork, 0)
	return artwork, nil
}

func (s *StatefulStore) SearchArtworks(ctx context.Context, filter ArtworkFilter, opts ...ArtworkSearchOptions) ([]*Artwork, error) {
	if len(filter.IDs) == 0 {
		return s.Store.SearchArtworks(ctx, filter, opts...)
	}

	var (
		opt      ArtworkSearchOptions
		artworks = make([]*Artwork, 0)
		newIDs   = make([]int, 0)
	)

	if len(opts) != 0 {
		opt = opts[0]
	} else {
		opt = DefaultSearchOptions()
	}

	for _, id := range filter.IDs {
		i, ok := s.cache.Get("artworks:" + strconv.Itoa(id))
		if !ok {
			newIDs = append(newIDs, id)
			continue
		}

		artworks = append(artworks, i.(*Artwork))
	}

	if len(newIDs) != 0 {
		filter.IDs = newIDs
		newArtworks, err := s.Store.SearchArtworks(ctx, filter, opts...)
		if err != nil {
			return nil, err
		}

		for _, artwork := range newArtworks {
			s.cache.Set("artworks:"+strconv.Itoa(artwork.ID), artwork, 0)
		}

		artworks = append(artworks, newArtworks...)
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
