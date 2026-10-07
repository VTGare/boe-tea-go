package post

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/artworks/twitter"
	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"

	"github.com/disgoorg/snowflake/v2"
	goCache "github.com/patrickmn/go-cache"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

func TestPost(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Post Suite")
}

type stubArtwork struct {
	id        string
	images    int
	renderErr error
}

func (*stubArtwork) StoreArtwork() *store.Artwork {
	return &store.Artwork{}
}

func (s *stubArtwork) Render() (artworks.Rendered, error) {
	if s.renderErr != nil {
		return artworks.Rendered{}, s.renderErr
	}

	rendered := artworks.Rendered{
		Title: "Art " + s.id,
		URL:   "https://example.com/" + s.id,
	}

	for i := 0; i < s.images; i++ {
		rendered.Images = append(rendered.Images, artworks.RenderedImage{
			Preview: "https://example.com/img.png",
		})
	}

	return rendered, nil
}

func (s *stubArtwork) ID() string {
	return s.id
}

func (s *stubArtwork) URL() string {
	return "https://example.com/" + s.id
}

func (s *stubArtwork) Len() int {
	return s.images
}

type stubProvider struct {
	mu     sync.Mutex
	ids    map[string]string
	arts   map[string]*stubArtwork
	fail   map[string]error
	delays map[string]time.Duration
	delay  time.Duration
	calls  []string
}

func (p *stubProvider) Match(url string) (string, bool) {
	id, ok := p.ids[url]

	return id, ok
}

func (p *stubProvider) Find(id string) (artworks.Artwork, error) {
	p.mu.Lock()
	p.calls = append(p.calls, id)
	p.mu.Unlock()

	if p.delay > 0 {
		time.Sleep(p.delay)
	}

	if d, ok := p.delays[id]; ok {
		time.Sleep(d)
	}

	if err, ok := p.fail[id]; ok {
		return nil, err
	}

	if art, ok := p.arts[id]; ok {
		return art, nil
	}

	return nil, errors.New("stub artwork not found")
}

func (*stubProvider) Info() artworks.Info {
	return artworks.Info{Key: "stub", Label: "Stub"}
}

type twitterStubProvider struct {
	art *twitter.Artwork
}

func (*twitterStubProvider) Match(string) (string, bool) {
	return "tweet", true
}

func (p *twitterStubProvider) Find(string) (artworks.Artwork, error) {
	return p.art, nil
}

func (*twitterStubProvider) Info() artworks.Info {
	return artworks.Info{Key: "twitter", Label: "Twitter"}
}

func (p *stubProvider) findCalls() []string {
	p.mu.Lock()
	defer p.mu.Unlock()

	return append([]string(nil), p.calls...)
}

type stubGuilds struct {
	guild *store.Guild
	err   error
}

func (s *stubGuilds) Guild(context.Context, string) (*store.Guild, error) {
	return s.guild, s.err
}

type deletedChannel struct {
	user    string
	group   string
	channel string
}

type stubUsers struct {
	mu      sync.Mutex
	user    *store.User
	err     error
	deleted []deletedChannel
}

func (s *stubUsers) User(context.Context, string) (*store.User, error) {
	return s.user, s.err
}

func (s *stubUsers) DeleteCrosspostChannel(_ context.Context, userID, group, channel string) (*store.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.deleted = append(s.deleted, deletedChannel{user: userID, group: group, channel: channel})

	return s.user, nil
}

func (s *stubUsers) deletions() []deletedChannel {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]deletedChannel(nil), s.deleted...)
}

type fakeDetector struct {
	mu      sync.Mutex
	found   map[string]*repost.Repost
	creates []*repost.Repost
	findErr error
}

func newFakeDetector() *fakeDetector {
	return &fakeDetector{found: make(map[string]*repost.Repost)}
}

func (*fakeDetector) key(channelID, artworkID string) string {
	return channelID + "\x00" + artworkID
}

func (f *fakeDetector) Find(_ context.Context, channelID, artworkID string) (*repost.Repost, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.findErr != nil {
		return nil, f.findErr
	}

	if rep, ok := f.found[f.key(channelID, artworkID)]; ok {
		return rep, nil
	}

	return nil, repost.ErrNotFound
}

func (f *fakeDetector) Create(_ context.Context, rep *repost.Repost, _ time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.creates = append(f.creates, rep)
	f.found[f.key(rep.ChannelID, rep.ID)] = rep

	return nil
}

func (f *fakeDetector) Delete(_ context.Context, channelID, artworkID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	delete(f.found, f.key(channelID, artworkID))

	return nil
}

func (*fakeDetector) Close() error {
	return nil
}

func (f *fakeDetector) created() []*repost.Repost {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]*repost.Repost(nil), f.creates...)
}

type testDeps struct {
	guilds   *stubGuilds
	users    *stubUsers
	detector *fakeDetector
	artCache *goCache.Cache
	fake     *sender.FakeSender

	mu          sync.Mutex
	recorded    []string
	match       func(string) (string, artworks.Provider)
	randomQuote func(bool) string
}

func newTestPoster() (*Poster, *testDeps) {
	deps := &testDeps{
		guilds:      &stubGuilds{guild: &store.Guild{ID: testGuildID.String(), Posting: store.Posting{Limit: 10}}},
		users:       &stubUsers{user: &store.User{ID: testAuthorID.String()}},
		detector:    newFakeDetector(),
		artCache:    goCache.New(5*time.Minute, 10*time.Minute),
		fake:        sender.NewFake(),
		randomQuote: func(bool) string { return "" },
	}

	poster := NewPoster(Deps{
		Guilds:       deps.guilds,
		Users:        deps.users,
		Match:        func(url string) (string, artworks.Provider) { return deps.matchURL(url) },
		Reposts:      deps.detector,
		ArtworkCache: deps.artCache,
		Sender:       deps.fake,
		Log:          zap.NewNop().Sugar(),
		RandomQuote:  func(nsfw bool) string { return deps.randomQuote(nsfw) },
		RecordArtwork: func(provider artworks.Provider) {
			deps.mu.Lock()
			defer deps.mu.Unlock()

			deps.recorded = append(deps.recorded, fmt.Sprintf("%T", provider))
		},
	})

	return poster, deps
}

func (d *testDeps) matchURL(url string) (string, artworks.Provider) {
	if d.match == nil {
		return "", nil
	}

	return d.match(url)
}

func matchProvider(provider artworks.Provider) func(string) (string, artworks.Provider) {
	return func(url string) (string, artworks.Provider) {
		if id, ok := provider.Match(url); ok {
			return id, provider
		}

		return "", nil
	}
}

func newTestRun(urls ...string) Post {
	return Post{
		GuildID:    testGuildID,
		ChannelID:  testChannelID,
		MessageID:  testMessageID,
		AuthorID:   testAuthorID,
		AuthorName: "tester",
		URLs:       urls,
		Skip:       SkipFilter{Indices: map[int]struct{}{}},
	}
}

func testItems(arts ...artworks.Artwork) []fetchedItem {
	items := make([]fetchedItem, 0, len(arts))
	for _, art := range arts {
		items = append(items, fetchedItem{artwork: art})
	}

	return items
}

const (
	testGuildID   snowflake.ID = 101
	testChannelID snowflake.ID = 201
	testMessageID snowflake.ID = 301
	testAuthorID  snowflake.ID = 401

	crossGuildID snowflake.ID = 102
	crossChannel snowflake.ID = 202
	pairChannel  snowflake.ID = 203
)

func crosspostGroup(children ...snowflake.ID) *store.Group {
	group := &store.Group{Name: "g", Parent: testChannelID.String()}
	for _, child := range children {
		group.Children = append(group.Children, child.String())
	}

	return group
}
