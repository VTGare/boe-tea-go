package store

import (
	"context"
	"errors"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	gocache "github.com/patrickmn/go-cache"
)

type stubBackend struct {
	Store

	guilds   map[string]*Guild
	artworks map[int]*Artwork

	guildErr   error
	artworkErr error

	guildReads   int
	artworkReads int
	searches     int
	lastFilter   ArtworkFilter

	bookmarkWon bool
	bookmarkErr error
}

func newStubBackend() *stubBackend {
	return &stubBackend{
		guilds:   make(map[string]*Guild),
		artworks: make(map[int]*Artwork),
	}
}

func newStatefulStub(backend *stubBackend) Store {
	return NewStatefulStore(backend, gocache.New(30*time.Minute, time.Hour))
}

func (s *stubBackend) Guild(_ context.Context, guildID string) (*Guild, error) {
	s.guildReads++

	if s.guildErr != nil {
		return nil, s.guildErr
	}

	guild, ok := s.guilds[guildID]
	if !ok {
		return nil, ErrGuildNotFound
	}

	return guild, nil
}

func (s *stubBackend) CreateGuild(_ context.Context, guildID string) (*Guild, error) {
	guild := DefaultGuild(guildID)
	s.guilds[guildID] = guild

	return guild, nil
}

func (s *stubBackend) UpdateGuild(_ context.Context, guild *Guild) (*Guild, error) {
	s.guilds[guild.ID] = guild

	return guild, nil
}

func (s *stubBackend) AddArtChannels(_ context.Context, guildID string, channels []string) (*Guild, error) {
	guild := s.guilds[guildID]
	guild.ArtChannels = append(guild.ArtChannels, channels...)

	return guild, nil
}

func (s *stubBackend) DeleteArtChannels(_ context.Context, guildID string, _ []string) (*Guild, error) {
	return s.guilds[guildID], nil
}

func (s *stubBackend) Artwork(_ context.Context, id int, _ string) (*Artwork, error) {
	s.artworkReads++

	if s.artworkErr != nil {
		return nil, s.artworkErr
	}

	artwork, ok := s.artworks[id]
	if !ok {
		return nil, ErrArtworkNotFound
	}

	return artwork, nil
}

func (s *stubBackend) SearchArtworks(_ context.Context, filter ArtworkFilter, _ ...ArtworkSearchOptions) ([]*Artwork, error) {
	s.searches++
	s.lastFilter = filter

	out := make([]*Artwork, 0, len(filter.IDs))
	for _, id := range filter.IDs {
		if artwork, ok := s.artworks[id]; ok {
			out = append(out, artwork)
		}
	}

	return out, nil
}

func (s *stubBackend) AddBookmark(_ context.Context, _ *Bookmark) (bool, error) {
	return s.bookmarkWon, s.bookmarkErr
}

func (s *stubBackend) DeleteBookmark(_ context.Context, _ *Bookmark) (bool, error) {
	return s.bookmarkWon, s.bookmarkErr
}

var _ = Describe("StatefulStore caching", func() {
	var (
		ctx     = context.Background()
		backend *stubBackend
		cached  Store
	)

	BeforeEach(func() {
		backend = newStubBackend()
		cached = newStatefulStub(backend)
	})

	It("serves guild reads from cache with owned copies", func() {
		backend.guilds["g"] = DefaultGuild("g")

		first, err := cached.Guild(ctx, "g")

		Expect(err).NotTo(HaveOccurred())

		second, err := cached.Guild(ctx, "g")

		Expect(err).NotTo(HaveOccurred())
		Expect(backend.guildReads).To(Equal(1))
		Expect(second).NotTo(BeIdenticalTo(first))
		Expect(second).To(Equal(first))
	})

	It("isolates callers from cached guilds in both directions", func() {
		backend.guilds["g"] = DefaultGuild("g")

		read, err := cached.Guild(ctx, "g")

		Expect(err).NotTo(HaveOccurred())

		read.Prefix = "xx!"
		read.ArtChannels = append(read.ArtChannels, "polluted")

		fresh, err := cached.Guild(ctx, "g")

		Expect(err).NotTo(HaveOccurred())
		Expect(fresh.Prefix).To(Equal("bt!"))
		Expect(fresh.ArtChannels).To(BeEmpty())

		written, err := cached.UpdateGuild(ctx, DefaultGuild("g"))

		Expect(err).NotTo(HaveOccurred())

		written.Prefix = "yy!"

		reread, err := cached.Guild(ctx, "g")

		Expect(err).NotTo(HaveOccurred())
		Expect(reread.Prefix).To(Equal("bt!"))
	})

	It("refreshes the cache on writes", func() {
		backend.guilds["g"] = DefaultGuild("g")

		_, err := cached.Guild(ctx, "g")

		Expect(err).NotTo(HaveOccurred())

		updated := DefaultGuild("g")
		updated.Prefix = "zz!"

		_, err = cached.UpdateGuild(ctx, updated)

		Expect(err).NotTo(HaveOccurred())

		reread, err := cached.Guild(ctx, "g")

		Expect(err).NotTo(HaveOccurred())
		Expect(reread.Prefix).To(Equal("zz!"))
		Expect(backend.guildReads).To(Equal(1))
	})

	It("does not cache misses", func() {
		backend.guildErr = errors.New("boom")

		_, err := cached.Guild(ctx, "g")
		Expect(err).To(HaveOccurred())

		_, err = cached.Guild(ctx, "g")
		Expect(err).To(HaveOccurred())

		Expect(backend.guildReads).To(Equal(2))
	})

	It("invalidates artwork on successful bookmark writes only", func() {
		backend.artworks[1] = &Artwork{ID: 1, Favorites: 1}
		backend.bookmarkWon = true

		_, err := cached.Artwork(ctx, 1, "")

		Expect(err).NotTo(HaveOccurred())
		Expect(backend.artworkReads).To(Equal(1))

		backend.bookmarkWon = false

		_, err = cached.AddBookmark(ctx, &Bookmark{ArtworkID: 1})

		Expect(err).NotTo(HaveOccurred())

		_, err = cached.Artwork(ctx, 1, "")

		Expect(err).NotTo(HaveOccurred())
		Expect(backend.artworkReads).To(Equal(1))

		backend.bookmarkWon = true

		_, err = cached.DeleteBookmark(ctx, &Bookmark{ArtworkID: 1})

		Expect(err).NotTo(HaveOccurred())

		_, err = cached.Artwork(ctx, 1, "")

		Expect(err).NotTo(HaveOccurred())
		Expect(backend.artworkReads).To(Equal(2))
	})
})

var _ = Describe("StatefulStore subset search", func() {
	var (
		ctx     = context.Background()
		backend *stubBackend
		cached  Store
	)

	BeforeEach(func() {
		backend = newStubBackend()
		backend.artworks[1] = &Artwork{ID: 1, Favorites: 1}
		backend.artworks[2] = &Artwork{ID: 2, Favorites: 3}
		backend.artworks[3] = &Artwork{ID: 3, Favorites: 2}

		cached = newStatefulStub(backend)
	})

	It("queries only uncached IDs without mutating the input filter", func() {
		_, err := cached.Artwork(ctx, 1, "")

		Expect(err).NotTo(HaveOccurred())

		filter := ArtworkFilter{IDs: []int{1, 2, 3}}

		results, err := cached.SearchArtworks(ctx, filter, ArtworkSearchOptions{Sort: ByPopularity, Order: Descending})

		Expect(err).NotTo(HaveOccurred())
		Expect(filter.IDs).To(Equal([]int{1, 2, 3}))
		Expect(backend.lastFilter.IDs).To(Equal([]int{2, 3}))
		Expect(results).To(HaveLen(3))
		Expect(results[0].ID).To(Equal(2))
		Expect(results[1].ID).To(Equal(3))
		Expect(results[2].ID).To(Equal(1))
	})

	It("passes empty filters straight through", func() {
		_, err := cached.SearchArtworks(ctx, ArtworkFilter{})

		Expect(err).NotTo(HaveOccurred())
		Expect(backend.searches).To(Equal(1))
	})
})
