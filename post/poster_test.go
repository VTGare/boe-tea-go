package post

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/VTGare/boe-tea-go/artworks/pixiv"
	"github.com/VTGare/boe-tea-go/artworks/render"
	"github.com/VTGare/boe-tea-go/artworks/twitter"
	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/VTGare/boe-tea-go/repost"
	"github.com/VTGare/boe-tea-go/store"

	"github.com/bwmarrin/discordgo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Generating messages", func() {
	It("returns no bundles without artworks", func() {
		poster := &Poster{}

		bundles, err := poster.generateMessages(&store.Guild{}, nil, Post{}, runOpts{})

		Expect(err).NotTo(HaveOccurred())
		Expect(bundles).To(BeEmpty())
	})
})

var _ = Describe("Skipping the first page", func() {
	var (
		twitterArtwork *twitter.Artwork
		pixivArtwork   = &pixiv.Artwork{}
		guild          = &store.Guild{SkipFirst: true}
	)

	BeforeEach(func() {
		guild = &store.Guild{SkipFirst: true}
		twitterArtwork = &twitter.Artwork{Photos: []string{"https://test.com/1.png"}}
	})

	It("should skip first if Twitter", func() {
		Expect(skipFirst(guild, twitterArtwork, runOpts{})).To(BeTrue())
	})

	It("shouldn't skip first if Twitter and command", func() {
		Expect(skipFirst(guild, twitterArtwork, runOpts{isCommand: true})).To(BeFalse())
	})

	It("should skip first if tweet has no images or videos", func() {
		twitterArtwork.Photos = []string{}
		Expect(skipFirst(guild, twitterArtwork, runOpts{})).To(BeTrue())
	})

	It("should skip first if tweet is NSFW", func() {
		twitterArtwork.NSFW = true
		Expect(skipFirst(guild, twitterArtwork, runOpts{})).To(BeTrue())
	})

	It("shouldn't skip first if SkipFirst setting is false", func() {
		guild.SkipFirst = false
		Expect(skipFirst(guild, twitterArtwork, runOpts{})).To(BeFalse())
	})

	It("shouldn't skip first if tweet has video", func() {
		twitterArtwork.Videos = []twitter.Video{{}}
		Expect(skipFirst(guild, twitterArtwork, runOpts{})).To(BeFalse())
	})

	It("shouldn't skip first if Twitter and crosspost", func() {
		Expect(skipFirst(guild, twitterArtwork, runOpts{isCrosspost: true})).To(BeFalse())
	})

	It("shouldn't skip first if not Twitter", func() {
		Expect(skipFirst(guild, pixivArtwork, runOpts{})).To(BeFalse())
	})
})

var _ = Describe("Album limits", func() {
	artwork := []*discordgo.MessageSend{{Content: "1"}, {Content: "2"}, {Content: "3"}, {Content: "4"}}

	bundles := func(sends ...[]*discordgo.MessageSend) []render.Bundle {
		out := make([]render.Bundle, 0, len(sends))
		for ind, s := range sends {
			out = append(out, render.Bundle{ID: strconv.Itoa(ind), Sends: s})
		}

		return out
	}

	It("should return the same array if limit not exceeded", func() {
		result := applyLimit(bundles(artwork), 4)
		Expect(result[0].Sends).To(HaveLen(4))
	})

	It("should cut over limit if one artwork", func() {
		result := applyLimit(bundles(artwork), 2)
		Expect(result[0].Sends).To(HaveLen(2))
		Expect(result[0].Sends[0].Content).To(
			Equal("Album size `(4)` exceeds the server's limit `(2)`, album has been cut."),
		)
	})

	It("should cut all except first page if two artworks", func() {
		result := applyLimit(bundles(artwork, artwork), 2)
		Expect(result).To(HaveLen(2))
		Expect(result[0].Sends).To(HaveLen(1))
		Expect(result[1].Sends).To(HaveLen(1))
		Expect(result[0].Sends[0].Content).To(
			Equal("Album size `(8)` exceeds the server's limit `(2)`, only the first image of every artwork has been sent."),
		)
	})

	It("should cut all except first page ignoring limit if more than one artwork", func() {
		result := applyLimit(bundles(artwork, artwork, artwork), 2)
		Expect(result).Should(HaveLen(3))
		Expect(result[0].Sends[0].Content).To(
			Equal("Album size `(12)` exceeds the server's limit `(2)`, only the first image of every artwork has been sent."),
		)
	})

	It("should keep each bundle ID with its first page", func() {
		result := applyLimit(bundles(artwork, artwork), 2)
		Expect(result[0].ID).To(Equal("0"))
		Expect(result[1].ID).To(Equal("1"))
	})
})

var _ = Describe("Skipping pages", func() {
	artworks := []*discordgo.MessageSend{
		{Content: "1"}, {Content: "2"}, {Content: "3"}, {Content: "4"},
	}

	It("should include first two", func() {
		result := applySkip(artworks, SkipFilter{
			Mode:    SkipModeInclude,
			Indices: map[int]struct{}{1: {}, 2: {}},
		})

		Expect(result).Should(And(
			HaveLen(2),
			ContainElements(&discordgo.MessageSend{Content: "1"}, &discordgo.MessageSend{Content: "2"}),
		))
	})

	It("should include last two", func() {
		result := applySkip(artworks, SkipFilter{
			Mode:    SkipModeInclude,
			Indices: map[int]struct{}{3: {}, 4: {}},
		})

		Expect(result).Should(And(
			HaveLen(2),
			ContainElements(&discordgo.MessageSend{Content: "3"}, &discordgo.MessageSend{Content: "4"}),
		))
	})

	It("should exclude last two", func() {
		result := applySkip(artworks, SkipFilter{
			Mode:    SkipModeExclude,
			Indices: map[int]struct{}{3: {}, 4: {}},
		})

		Expect(result).Should(And(
			HaveLen(2),
			ContainElements(&discordgo.MessageSend{Content: "1"}, &discordgo.MessageSend{Content: "2"}),
		))
	})

	It("shouldn't exclude anything", func() {
		result := applySkip(artworks, SkipFilter{
			Mode:    SkipModeExclude,
			Indices: map[int]struct{}{5: {}},
		})

		Expect(result).Should(And(
			HaveLen(4),
			ContainElements(
				&discordgo.MessageSend{Content: "1"}, &discordgo.MessageSend{Content: "2"},
				&discordgo.MessageSend{Content: "3"}, &discordgo.MessageSend{Content: "4"},
			),
		))
	})

	It("shouldn't include anything", func() {
		result := applySkip(artworks, SkipFilter{
			Mode:    SkipModeInclude,
			Indices: map[int]struct{}{5: {}},
		})

		Expect(result).Should(HaveLen(0))
	})
})

var _ = Describe("Imageless tweets", func() {
	var (
		poster   *Poster
		deps     *testDeps
		provider *twitterStubProvider
	)

	BeforeEach(func() {
		poster, deps = newTestPoster()
		provider = &twitterStubProvider{art: &twitter.Artwork{}}
		deps.match = matchProvider(provider)
	})

	It("drops imageless tweets outside commands", func() {
		sent, err := poster.Send(context.Background(), newTestRun("https://example.com/t"))

		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(BeEmpty())
		Expect(deps.fake.Complex).To(BeEmpty())
		Expect(deps.detector.created()).To(BeEmpty())
	})

	It("serves imageless tweets from commands", func() {
		run := newTestRun("https://example.com/t")
		run.IsCommand = true

		sent, err := poster.Send(context.Background(), run)

		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(1))
	})

	It("still serves photo tweets outside commands", func() {
		provider.art = &twitter.Artwork{FullName: "smth", Photos: []string{"https://example.com/1.png"}}

		sent, err := poster.Send(context.Background(), newTestRun("https://example.com/t"))

		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(1))
	})
})

func restError(status int) error {
	return &discordgo.RESTError{Response: &http.Response{StatusCode: status}}
}

var _ = Describe("Classify", func() {
	It("marks skips as no-perms", func() {
		Expect(classify(sender.ErrSkipped)).To(Equal(KindNoPerms))
	})

	It("marks forbidden responses as no-perms", func() {
		Expect(classify(restError(http.StatusForbidden))).To(Equal(KindNoPerms))
		Expect(classify(restError(http.StatusUnauthorized))).To(Equal(KindNoPerms))
	})

	It("marks everything else transient", func() {
		Expect(classify(restError(http.StatusInternalServerError))).To(Equal(KindTransient))
		Expect(classify(restError(http.StatusTooManyRequests))).To(Equal(KindTransient))
		Expect(classify(errors.New("boom"))).To(Equal(KindTransient))
	})

	It("wraps send failures with their kind and unwraps to the cause", func() {
		cause := restError(http.StatusForbidden)
		err := &Error{Kind: classify(cause), Cause: cause}

		Expect(err.Kind).To(Equal(KindNoPerms))

		var restErr *discordgo.RESTError
		Expect(errors.As(err, &restErr)).To(BeTrue())
	})
})

var _ = Describe("Missing send permissions", func() {
	var (
		poster *Poster
		deps   *testDeps
		guild  = &store.Guild{ID: "guild-1"}
	)

	BeforeEach(func() {
		poster, deps = newTestPoster()
		_ = deps
	})

	It("skips the fetch without permissions", func() {
		deps.fake.ChannelPerms = false

		results, err := poster.fetch(context.Background(), guild, "channel-1", nil, runOpts{})

		Expect(err).NotTo(HaveOccurred())
		Expect(results.items).To(BeEmpty())
		Expect(results.reposts).To(BeEmpty())
	})

	It("returns empty results with permissions and no urls", func() {
		results, err := poster.fetch(context.Background(), guild, "channel-1", nil, runOpts{})

		Expect(err).NotTo(HaveOccurred())
		Expect(results.items).To(BeEmpty())
	})
})

var _ = Describe("Delivering artwork", func() {
	var (
		poster *Poster
		deps   *testDeps
		guild  = &store.Guild{ID: "guild-1", Limit: 10}
		run    = newTestRun()
	)

	BeforeEach(func() {
		poster, deps = newTestPoster()
		_ = deps
	})

	It("records one bundle per artwork with its artwork ID", func() {
		pages, err := poster.deliver(guild, "channel-1", testItems(
			&stubArtwork{id: "a", images: 1},
			&stubArtwork{id: "b", images: 2},
		), run, runOpts{})

		Expect(err).NotTo(HaveOccurred())
		Expect(pages).To(HaveLen(3))
		Expect(pages[0].info.ArtworkID).To(Equal("a"))
		Expect(pages[1].info.ArtworkID).To(Equal("b"))
		Expect(pages[2].info.ArtworkID).To(Equal("b"))
		Expect(deps.fake.Complex).To(HaveLen(3))
		Expect(deps.fake.Complex[0].ChannelID).To(Equal("channel-1"))
	})

	It("drops every message when the sender skips", func() {
		deps.fake.Skip = true

		pages, err := poster.deliver(guild, "channel-1", testItems(
			&stubArtwork{id: "a", images: 1},
		), run, runOpts{})

		Expect(err).NotTo(HaveOccurred())
		Expect(pages).To(BeEmpty())
		Expect(deps.fake.Complex).To(BeEmpty())
	})

	It("joins send failures instead of swallowing them", func() {
		deps.fake.SendErr = errors.New("boom")

		pages, err := poster.deliver(guild, "channel-1", testItems(
			&stubArtwork{id: "a", images: 1},
		), run, runOpts{})

		Expect(err).To(HaveOccurred())
		Expect(pages).To(BeEmpty())
	})

	It("adds bookmark reactions in finalize when the guild wants them", func() {
		guild.Reactions = true
		defer func() { guild.Reactions = false }()

		pages, err := poster.deliver(guild, "channel-1", testItems(
			&stubArtwork{id: "a", images: 1},
		), run, runOpts{})

		Expect(err).NotTo(HaveOccurred())
		Expect(deps.fake.Reactions).To(BeEmpty())

		fetched := fetchResults{items: testItems(&stubArtwork{id: "a", images: 1})}

		Expect(poster.finalize(run, guild, pages, fetched, runOpts{})).To(Succeed())
		Expect(deps.fake.Reactions).To(HaveLen(2))
	})

	It("references the triggering message on every page", func() {
		_, err := poster.deliver(guild, "channel-1", testItems(
			&stubArtwork{id: "a", images: 1},
		), run, runOpts{})

		Expect(err).NotTo(HaveOccurred())
		Expect(deps.fake.Complex).To(HaveLen(1))
		Expect(deps.fake.Complex[0].Message.Reference.MessageID).To(Equal("event-1"))
	})
})

var _ = Describe("Repost notices", func() {
	var (
		poster *Poster
		deps   *testDeps
		guild  = &store.Guild{ID: "guild-1", Repost: store.GuildRepostStrict}
		run    = newTestRun()
		reps   = []*repost.Repost{{ID: "a", URL: "https://example.com/a"}}
	)

	BeforeEach(func() {
		poster, deps = newTestPoster()
		_ = deps
	})

	It("deletes the original, sends the notice, and expires it", func() {
		Expect(poster.notifyReposts(guild, run, reps, 1)).To(Succeed())

		Expect(deps.fake.Deleted).To(HaveLen(1))
		Expect(deps.fake.Deleted[0].MessageID).To(Equal("event-1"))
		Expect(deps.fake.Embeds).To(HaveLen(1))
		Expect(deps.fake.Expired).To(HaveLen(1))
	})

	It("keeps the original without delete permissions but still warns", func() {
		deps.fake.GuildPerms = false

		Expect(poster.notifyReposts(guild, run, reps, 1)).To(Succeed())

		Expect(deps.fake.Deleted).To(BeEmpty())
		Expect(deps.fake.Embeds).To(HaveLen(1))
		Expect(deps.fake.Expired).To(HaveLen(1))
	})

	It("does nothing without reposts", func() {
		Expect(poster.notifyReposts(guild, run, nil, 0)).To(Succeed())

		Expect(deps.fake.Deleted).To(BeEmpty())
		Expect(deps.fake.Embeds).To(BeEmpty())
		Expect(deps.fake.Expired).To(BeEmpty())
	})
})

var _ = Describe("Keeping artwork with its pages", func() {
	It("keeps each ID with its own pages without positional coupling", func() {
		poster, _ := newTestPoster()
		guild := &store.Guild{ID: "guild-1"}

		bundles, err := poster.generateMessages(guild, testItems(
			&stubArtwork{id: "a", images: 1},
			&stubArtwork{id: "b", images: 2},
		), newTestRun(), runOpts{})

		Expect(err).NotTo(HaveOccurred())
		Expect(bundles).To(HaveLen(2))
		Expect(bundles[0].ID).To(Equal("a"))
		Expect(bundles[0].Sends).To(HaveLen(1))
		Expect(bundles[1].ID).To(Equal("b"))
		Expect(bundles[1].Sends).To(HaveLen(2))
	})
})

var _ = Describe("Crossposting", func() {
	var (
		poster *Poster
		deps   *testDeps
		run    = newTestRun()
	)

	BeforeEach(func() {
		poster, deps = newTestPoster()

		deps.fake.ChannelGuilds = map[string]string{"ch1": "g1", "ch2": "g1"}
		deps.fake.Members = map[sender.MemberKey]bool{{GuildID: "g1", UserID: "u1"}: true}
		deps.users.user = &store.User{ID: "u1"}
	})

	It("sends to member channels without touching the group", func() {
		sent, err := poster.Crosspost(context.Background(), run, "u1", crosspostGroup("ch1", "ch2"))

		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(BeEmpty())
		Expect(deps.users.deletions()).To(BeEmpty())
	})

	It("removes channels the member left", func() {
		delete(deps.fake.Members, sender.MemberKey{GuildID: "g1", UserID: "u1"})

		_, err := poster.Crosspost(context.Background(), run, "u1", crosspostGroup("ch1"))

		Expect(err).NotTo(HaveOccurred())
		Expect(deps.users.deletions()).To(HaveLen(1))
		Expect(deps.users.deletions()[0].channel).To(Equal("ch1"))
	})

	It("keeps channels on membership lookup failures", func() {
		deps.fake.MemberErr = errors.New("boom")

		_, err := poster.Crosspost(context.Background(), run, "u1", crosspostGroup("ch1"))

		Expect(err).NotTo(HaveOccurred())
		Expect(deps.users.deletions()).To(BeEmpty())
	})

	It("skips unresolvable channels", func() {
		deps.fake.ChannelErr = errors.New("boom")

		_, err := poster.Crosspost(context.Background(), run, "u1", crosspostGroup("ch1"))

		Expect(err).NotTo(HaveOccurred())
		Expect(deps.users.deletions()).To(BeEmpty())
	})

	It("delivers fetched artworks to member channels in order", func() {
		provider := &stubProvider{
			ids:     map[string]string{"https://example.com/a": "a", "https://example.com/b": "b"},
			arts:    map[string]*stubArtwork{"a": {id: "a", images: 1}, "b": {id: "b", images: 1}},
			enabled: true,
		}

		deps.match = matchProvider(provider)
		deps.guilds.guild = &store.Guild{ID: "g1", Limit: 10, Crosspost: true}

		run.URLs = []string{"https://example.com/a", "https://example.com/b"}

		sent, err := poster.Crosspost(context.Background(), run, "u1", crosspostGroup("ch1"))

		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(2))
		Expect(sent[0].ArtworkID).To(Equal("a"))
		Expect(sent[1].ArtworkID).To(Equal("b"))
		Expect(deps.fake.Complex).To(HaveLen(2))
		Expect(deps.fake.Complex[0].ChannelID).To(Equal("ch1"))
	})
})

var _ = Describe("Sending posts", func() {
	var (
		poster *Poster
		deps   *testDeps
	)

	BeforeEach(func() {
		poster, deps = newTestPoster()
	})

	It("sends matched artworks and returns their infos", func() {
		provider := &stubProvider{
			ids:     map[string]string{"https://example.com/a": "a"},
			arts:    map[string]*stubArtwork{"a": {id: "a", images: 1}},
			enabled: true,
		}

		deps.match = matchProvider(provider)

		sent, err := poster.Send(context.Background(), newTestRun("https://example.com/a"))

		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(1))
		Expect(sent[0].ArtworkID).To(Equal("a"))
		Expect(deps.detector.created()).To(HaveLen(1))
	})

	It("stays silent for ignored users outside commands", func() {
		deps.users.user = &store.User{ID: "author-1", Ignore: true}

		sent, err := poster.Send(context.Background(), newTestRun("https://example.com/a"))

		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(BeEmpty())
		Expect(deps.fake.Complex).To(BeEmpty())
	})

	It("still serves ignored users from commands", func() {
		provider := &stubProvider{
			ids:     map[string]string{"https://example.com/a": "a"},
			arts:    map[string]*stubArtwork{"a": {id: "a", images: 1}},
			enabled: true,
		}

		deps.match = matchProvider(provider)
		deps.users.user = &store.User{ID: "author-1", Ignore: true}

		run := newTestRun("https://example.com/a")
		run.IsCommand = true

		sent, err := poster.Send(context.Background(), run)

		Expect(err).NotTo(HaveOccurred())
		Expect(sent).To(HaveLen(1))
	})

	It("records stats once per fetched artwork", func() {
		provider := &stubProvider{
			ids:     map[string]string{"https://example.com/a": "a", "https://example.com/b": "b"},
			arts:    map[string]*stubArtwork{"a": {id: "a", images: 1}, "b": {id: "b", images: 1}},
			enabled: true,
		}

		deps.match = matchProvider(provider)

		_, err := poster.Send(context.Background(), newTestRun("https://example.com/a", "https://example.com/b"))

		Expect(err).NotTo(HaveOccurred())
		Expect(deps.recorded).To(HaveLen(2))
	})
})

var _ = Describe("Failed renders", func() {
	var (
		poster   *Poster
		deps     *testDeps
		provider *stubProvider
	)

	BeforeEach(func() {
		poster, deps = newTestPoster()

		provider = &stubProvider{
			ids:     map[string]string{"https://example.com/poison": "poison"},
			arts:    map[string]*stubArtwork{"poison": {id: "poison", images: 1, renderErr: errors.New("render boom")}},
			enabled: true,
		}

		deps.match = matchProvider(provider)
	})

	It("aborts Send without side effects", func() {
		deps.detector.found["channel-1\x00poison"] = &repost.Repost{ID: "poison", URL: "https://example.com/poison"}
		deps.guilds.guild.Repost = store.GuildRepostEnabled
		deps.guilds.guild.Crosspost = true
		deps.users.user.Groups = []*store.Group{{Name: "g", Parent: "channel-1", Children: []string{"ch1"}}}
		deps.users.user.Crosspost = true
		deps.fake.ChannelGuilds = map[string]string{"ch1": "g1"}
		deps.fake.Members = map[sender.MemberKey]bool{{GuildID: "g1", UserID: "author-1"}: true}

		sent, err := poster.Send(context.Background(), newTestRun("https://example.com/poison"))

		Expect(err).To(HaveOccurred())
		Expect(sent).To(BeEmpty())
		Expect(deps.fake.Complex).To(BeEmpty())
		Expect(deps.fake.Embeds).To(BeEmpty())
		Expect(deps.fake.Reactions).To(BeEmpty())
		Expect(deps.recorded).To(BeEmpty())
	})

	It("aborts one crosspost channel without touching the rest", func() {
		deps.guilds.guild = &store.Guild{ID: "g1", Limit: 10, Crosspost: true}
		deps.fake.ChannelGuilds = map[string]string{"ch1": "g1"}
		deps.fake.Members = map[sender.MemberKey]bool{{GuildID: "g1", UserID: "u1"}: true}
		deps.users.user = &store.User{ID: "u1"}

		sent, err := poster.Crosspost(context.Background(), newTestRun("https://example.com/poison"), "u1", crosspostGroup("ch1"))

		Expect(err).To(HaveOccurred())
		Expect(sent).To(BeEmpty())
		Expect(deps.fake.Complex).To(BeEmpty())
	})
})

var _ = Describe("Recording reposts", func() {
	var (
		poster   *Poster
		deps     *testDeps
		provider *stubProvider
		guild    = &store.Guild{ID: "guild-1", Limit: 10, Repost: store.GuildRepostEnabled}
	)

	BeforeEach(func() {
		poster, deps = newTestPoster()

		provider = &stubProvider{
			ids: map[string]string{
				"https://example.com/good": "good",
				"https://example.com/bad":  "bad",
			},
			arts:    map[string]*stubArtwork{"good": {id: "good", images: 1}},
			fail:    map[string]error{"bad": errors.New("provider boom")},
			enabled: true,
		}

		deps.match = matchProvider(provider)
	})

	It("records a repost only for successful fetches", func() {
		results, err := poster.fetch(context.Background(), guild, "channel-1",
			[]string{"https://example.com/good", "https://example.com/bad"}, runOpts{messageID: "event-1"})

		Expect(err).To(HaveOccurred())
		Expect(results.items).To(HaveLen(1))
		Expect(results.items[0].artwork.ID()).To(Equal("good"))
		Expect(deps.detector.created()).To(HaveLen(1))
		Expect(deps.detector.created()[0].ID).To(Equal("good"))
	})

	It("never records a repost when every fetch fails", func() {
		_, err := poster.fetch(context.Background(), guild, "channel-1",
			[]string{"https://example.com/bad"}, runOpts{messageID: "event-1"})

		Expect(err).To(HaveOccurred())
		Expect(deps.detector.created()).To(BeEmpty())
	})

	It("keeps reporting known reposts without re-recording them", func() {
		deps.detector.found["channel-1\x00good"] = &repost.Repost{ID: "good", URL: "https://example.com/good"}

		results, err := poster.fetch(context.Background(), guild, "channel-1",
			[]string{"https://example.com/good"}, runOpts{messageID: "event-1"})

		Expect(err).NotTo(HaveOccurred())
		Expect(results.items).To(HaveLen(1))
		Expect(results.reposts).To(HaveLen(1))
		Expect(deps.detector.created()).To(BeEmpty())
	})
})

var _ = Describe("Leaving the input group alone", func() {
	var (
		poster *Poster
		deps   *testDeps
		run    = newTestRun()
	)

	BeforeEach(func() {
		poster, deps = newTestPoster()

		deps.fake.ChannelGuilds = map[string]string{"ch1": "g1", "ch2": "g1"}
		deps.fake.Members = map[sender.MemberKey]bool{{GuildID: "g1", UserID: "u1"}: true}
		deps.users.user = &store.User{ID: "u1"}
	})

	It("never mutates the input group", func() {
		group := &store.Group{
			Name:     "g",
			Parent:   "parent",
			Children: []string{"channel-1", "ch1", "ch2"},
			IsPair:   true,
		}

		run.ExcludedChannels = []string{"ch2"}

		_, err := poster.Crosspost(context.Background(), run, "u1", group)

		Expect(err).NotTo(HaveOccurred())
		Expect(group.Children).To(Equal([]string{"channel-1", "ch1", "ch2"}))
		Expect(deps.users.deletions()).To(BeEmpty())
	})
})

var _ = Describe("Fetching in order, once each", func() {
	var (
		poster   *Poster
		deps     *testDeps
		provider *stubProvider
		guild    = &store.Guild{ID: "guild-1", Limit: 10}
	)

	BeforeEach(func() {
		poster, deps = newTestPoster()

		provider = &stubProvider{
			ids: map[string]string{
				"https://example.com/slow": "slow",
				"https://example.com/fast": "fast",
			},
			arts: map[string]*stubArtwork{
				"slow": {id: "slow", images: 1},
				"fast": {id: "fast", images: 1},
			},
			delays:  map[string]time.Duration{"slow": 80 * time.Millisecond},
			enabled: true,
		}

		deps.match = matchProvider(provider)
	})

	It("keeps input-URL order despite uneven fetch latency", func() {
		results, err := poster.fetch(context.Background(), guild, "channel-1",
			[]string{"https://example.com/slow", "https://example.com/fast"}, runOpts{})

		Expect(err).NotTo(HaveOccurred())
		Expect(results.items).To(HaveLen(2))
		Expect(results.items[0].artwork.ID()).To(Equal("slow"))
		Expect(results.items[1].artwork.ID()).To(Equal("fast"))
		Expect(results.matched).To(Equal(2))
	})

	It("fetches a duplicated artwork ID once", func() {
		results, err := poster.fetch(context.Background(), guild, "channel-1",
			[]string{"https://example.com/slow", "https://example.com/slow"}, runOpts{})

		Expect(err).NotTo(HaveOccurred())
		Expect(results.items).To(HaveLen(1))
		Expect(results.matched).To(Equal(1))
		Expect(provider.findCalls()).To(HaveLen(1))
	})
})
