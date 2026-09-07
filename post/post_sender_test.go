package post

import (
	"context"
	"errors"
	"sync"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/gumi"

	"github.com/bwmarrin/discordgo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/zap"
)

type stubArtwork struct {
	id     string
	images int
}

func (*stubArtwork) StoreArtwork() *store.Artwork {
	return &store.Artwork{}
}

func (s *stubArtwork) Render() (artworks.Rendered, error) {
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

func newSenderPost(s sender.Sender) *Post {
	return &Post{
		Bot:    &bot.Bot{Log: zap.NewNop().Sugar()},
		Sender: s,
		Ctx: &gumi.Ctx{Event: &discordgo.MessageCreate{Message: &discordgo.Message{
			ID:        "event-1",
			ChannelID: "channel-1",
			GuildID:   "guild-1",
			Author:    &discordgo.User{ID: "author-1", Username: "tester"},
		}}},
	}
}

var _ = Describe("Fetch permission gate", func() {
	var (
		fake  *sender.FakeSender
		guild = &store.Guild{ID: "guild-1"}
	)

	BeforeEach(func() {
		fake = sender.NewFake()
	})

	It("skips the fetch without permissions", func() {
		fake.ChannelPerms = false
		p := newSenderPost(fake)

		results, err := p.fetch(context.Background(), guild, "channel-1")

		Expect(err).NotTo(HaveOccurred())
		Expect(results.artworks).To(BeEmpty())
		Expect(results.reposts).To(BeEmpty())
	})

	It("returns empty results with permissions and no urls", func() {
		p := newSenderPost(fake)

		results, err := p.fetch(context.Background(), guild, "channel-1")

		Expect(err).NotTo(HaveOccurred())
		Expect(results.artworks).To(BeEmpty())
	})
})

var _ = Describe("SendMessages through Sender", func() {
	var (
		fake  *sender.FakeSender
		guild = &store.Guild{ID: "guild-1", Limit: 10}
	)

	BeforeEach(func() {
		fake = sender.NewFake()
	})

	It("records one bundle per artwork with its artwork ID", func() {
		p := newSenderPost(fake)

		sent, err := p.sendMessages(guild, "channel-1", []artworks.Artwork{
			&stubArtwork{id: "a", images: 1},
			&stubArtwork{id: "b", images: 2},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(3))
		Expect(sent[0].ArtworkID).To(Equal("a"))
		Expect(sent[1].ArtworkID).To(Equal("b"))
		Expect(sent[2].ArtworkID).To(Equal("b"))
		Expect(fake.Complex).To(HaveLen(3))
		Expect(fake.Complex[0].ChannelID).To(Equal("channel-1"))
	})

	It("drops every message when the sender skips", func() {
		fake.Skip = true
		p := newSenderPost(fake)

		sent, err := p.sendMessages(guild, "channel-1", []artworks.Artwork{
			&stubArtwork{id: "a", images: 1},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(BeEmpty())
		Expect(fake.Complex).To(BeEmpty())
	})

	It("adds bookmark reactions when the guild wants them", func() {
		p := newSenderPost(fake)
		guild.Reactions = true
		defer func() { guild.Reactions = false }()

		_, err := p.sendMessages(guild, "channel-1", []artworks.Artwork{
			&stubArtwork{id: "a", images: 1},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(fake.Reactions).To(HaveLen(2))
	})

	It("references the triggering message on every page", func() {
		p := newSenderPost(fake)

		_, err := p.sendMessages(guild, "channel-1", []artworks.Artwork{
			&stubArtwork{id: "a", images: 1},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(fake.Complex).To(HaveLen(1))
		Expect(fake.Complex[0].Message.Reference.MessageID).To(Equal("event-1"))
	})
})

var _ = Describe("HandleReposts through Sender", func() {
	var (
		fake  *sender.FakeSender
		guild = &store.Guild{ID: "guild-1", Repost: store.GuildRepostStrict}
		reps  = []*repost.Repost{{ID: "a", URL: "https://example.com/a"}}
	)

	BeforeEach(func() {
		fake = sender.NewFake()
	})

	It("deletes the original, sends the notice, and expires it", func() {
		p := newSenderPost(fake)
		p.handleReposts(guild, reps, 1)

		Expect(fake.Deleted).To(HaveLen(1))
		Expect(fake.Deleted[0].MessageID).To(Equal("event-1"))
		Expect(fake.Embeds).To(HaveLen(1))
		Expect(fake.Expired).To(HaveLen(1))
	})

	It("keeps the original without delete permissions but still warns", func() {
		fake.GuildPerms = false
		p := newSenderPost(fake)
		p.handleReposts(guild, reps, 1)

		Expect(fake.Deleted).To(BeEmpty())
		Expect(fake.Embeds).To(HaveLen(1))
		Expect(fake.Expired).To(HaveLen(1))
	})

	It("does nothing without reposts", func() {
		p := newSenderPost(fake)
		p.handleReposts(guild, nil, 0)

		Expect(fake.Deleted).To(BeEmpty())
		Expect(fake.Embeds).To(BeEmpty())
		Expect(fake.Expired).To(BeEmpty())
	})
})

var _ = Describe("GenerateMessages correlation", func() {
	It("keeps each ID with its own pages without positional coupling", func() {
		p := newSenderPost(sender.NewFake())
		guild := &store.Guild{ID: "guild-1"}

		bundles, err := p.generateMessages(guild, []artworks.Artwork{
			&stubArtwork{id: "a", images: 1},
			&stubArtwork{id: "b", images: 2},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(bundles).To(HaveLen(2))
		Expect(bundles[0].ID).To(Equal("a"))
		Expect(bundles[0].Sends).To(HaveLen(1))
		Expect(bundles[1].ID).To(Equal("b"))
		Expect(bundles[1].Sends).To(HaveLen(2))
	})
})

type deletedChannel struct {
	user    string
	group   string
	channel string
}

type crosspostStub struct {
	store.Store

	mu      sync.Mutex
	user    *store.User
	deleted []deletedChannel
}

func (s *crosspostStub) User(_ context.Context, _ string) (*store.User, error) {
	return s.user, nil
}

func (*crosspostStub) Guild(_ context.Context, guildID string) (*store.Guild, error) {
	return &store.Guild{ID: guildID, Crosspost: true}, nil
}

func (s *crosspostStub) DeleteCrosspostChannel(_ context.Context, userID, group, channel string) (*store.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.deleted = append(s.deleted, deletedChannel{user: userID, group: group, channel: channel})

	return s.user, nil
}

func (s *crosspostStub) deletions() []deletedChannel {
	s.mu.Lock()
	defer s.mu.Unlock()

	return append([]deletedChannel(nil), s.deleted...)
}

func newCrosspostPost(s sender.Sender, backend *crosspostStub) *Post {
	p := newSenderPost(s)
	p.Bot.Store = backend

	return p
}

func crosspostGroup(children ...string) *store.Group {
	return &store.Group{Name: "g", Parent: "parent", Children: children}
}

var _ = Describe("Crosspost fan-out", func() {
	var (
		fake    *sender.FakeSender
		backend *crosspostStub
	)

	BeforeEach(func() {
		fake = sender.NewFake()
		fake.ChannelGuilds = map[string]string{"ch1": "g1", "ch2": "g1"}
		fake.Members = map[sender.MemberKey]bool{{GuildID: "g1", UserID: "u1"}: true}

		backend = &crosspostStub{
			user: &store.User{ID: "u1"},
		}
	})

	It("sends to member channels without touching the group", func() {
		p := newCrosspostPost(fake, backend)

		sent, err := p.Crosspost(context.Background(), "u1", crosspostGroup("ch1", "ch2"))

		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(BeEmpty())
		Expect(backend.deletions()).To(BeEmpty())
	})

	It("removes channels the member left", func() {
		delete(fake.Members, sender.MemberKey{GuildID: "g1", UserID: "u1"})
		p := newCrosspostPost(fake, backend)

		_, err := p.Crosspost(context.Background(), "u1", crosspostGroup("ch1"))

		Expect(err).NotTo(HaveOccurred())
		Expect(backend.deletions()).To(HaveLen(1))
		Expect(backend.deletions()[0].channel).To(Equal("ch1"))
	})

	It("keeps channels on membership lookup failures", func() {
		fake.MemberErr = errors.New("boom")
		p := newCrosspostPost(fake, backend)

		_, err := p.Crosspost(context.Background(), "u1", crosspostGroup("ch1"))

		Expect(err).NotTo(HaveOccurred())
		Expect(backend.deletions()).To(BeEmpty())
	})

	It("skips unresolvable channels", func() {
		fake.ChannelErr = errors.New("boom")
		p := newCrosspostPost(fake, backend)

		_, err := p.Crosspost(context.Background(), "u1", crosspostGroup("ch1"))

		Expect(err).NotTo(HaveOccurred())
		Expect(backend.deletions()).To(BeEmpty())
	})
})
