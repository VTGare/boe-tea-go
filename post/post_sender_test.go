package post

import (
	"context"

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
	id    string
	sends []*discordgo.MessageSend
}

func (*stubArtwork) StoreArtwork() *store.Artwork {
	return &store.Artwork{}
}

func (s *stubArtwork) MessageSends(_ string, _ bool) ([]*discordgo.MessageSend, error) {
	return s.sends, nil
}

func (s *stubArtwork) ID() string {
	return s.id
}

func (s *stubArtwork) URL() string {
	return "https://example.com/" + s.id
}

func (s *stubArtwork) Len() int {
	return len(s.sends)
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

	artwork := func(id string) artworks.Artwork {
		return &stubArtwork{
			id: id,
			sends: []*discordgo.MessageSend{{
				Content: "art " + id,
				Embeds:  []*discordgo.MessageEmbed{{URL: "https://example.com/" + id}},
			}},
		}
	}

	It("records one send per artwork with its artwork ID", func() {
		p := newSenderPost(fake)

		sent, err := p.sendMessages(guild, "channel-1", []artworks.Artwork{artwork("a"), artwork("b")})

		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(2))
		Expect(sent[0].ArtworkID).To(Equal("a"))
		Expect(sent[1].ArtworkID).To(Equal("b"))
		Expect(fake.Complex).To(HaveLen(2))
		Expect(fake.Complex[0].ChannelID).To(Equal("channel-1"))
	})

	It("drops every message when the sender skips", func() {
		fake.Skip = true
		p := newSenderPost(fake)

		sent, err := p.sendMessages(guild, "channel-1", []artworks.Artwork{artwork("a")})

		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(BeEmpty())
		Expect(fake.Complex).To(BeEmpty())
	})

	It("adds bookmark reactions when the guild wants them", func() {
		p := newSenderPost(fake)
		guild.Reactions = true
		defer func() { guild.Reactions = false }()

		_, err := p.sendMessages(guild, "channel-1", []artworks.Artwork{artwork("a")})

		Expect(err).NotTo(HaveOccurred())
		Expect(fake.Reactions).To(HaveLen(2))
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
