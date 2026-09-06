package post

import (
	"strconv"
	"testing"

	"github.com/VTGare/boe-tea-go/artworks/pixiv"
	"github.com/VTGare/boe-tea-go/artworks/render"
	"github.com/VTGare/boe-tea-go/artworks/twitter"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/gumi"

	"github.com/bwmarrin/discordgo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPost(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Post Suite")
}

var _ = Describe("Generate Messages Tests", func() {
	var post Post

	It("returns no bundles without artworks", func() {
		post = Post{Ctx: &gumi.Ctx{Event: &discordgo.MessageCreate{Message: &discordgo.Message{}}}}

		bundles, err := post.generateMessages(&store.Guild{}, nil)

		Expect(err).NotTo(HaveOccurred())
		Expect(bundles).To(BeEmpty())
	})
})

var _ = Describe("Skip First Tests", func() {
	var (
		post           Post
		twitterArtwork *twitter.Artwork
		pixivArtwork   = &pixiv.Artwork{}
		guild          = &store.Guild{SkipFirst: true}
	)

	BeforeEach(func() {
		post = Post{Ctx: &gumi.Ctx{}}
		guild = &store.Guild{SkipFirst: true}
		twitterArtwork = &twitter.Artwork{Photos: []string{"https://test.com/1.png"}}
	})

	It("should skip first if Twitter", func() {
		Expect(post.skipFirst(guild, twitterArtwork)).To(BeTrue())
	})

	It("shouldn't skip first if Twitter and command", func() {
		post.Ctx.Command = &gumi.Command{}
		Expect(post.skipFirst(guild, twitterArtwork)).To(BeFalse())
	})

	It("should skip first if tweet has no images or videos", func() {
		twitterArtwork.Photos = []string{}
		Expect(post.skipFirst(guild, twitterArtwork)).To(BeTrue())
	})

	It("should skip first if tweet is NSFW", func() {
		twitterArtwork.NSFW = true
		Expect(post.skipFirst(guild, twitterArtwork)).To(BeTrue())
	})

	It("shouldn't skip first if SkipFirst setting is false", func() {
		guild.SkipFirst = false
		Expect(post.skipFirst(guild, twitterArtwork)).To(BeFalse())
	})

	It("shouldn't skip first if tweet has video", func() {
		twitterArtwork.Videos = []twitter.Video{{}}
		Expect(post.skipFirst(guild, twitterArtwork)).To(BeFalse())
	})

	It("shouldn't skip first if Twitter and crosspost", func() {
		post.CrosspostMode = true
		Expect(post.skipFirst(guild, twitterArtwork)).To(BeFalse())
	})

	It("shouldn't skip first if not Twitter", func() {
		Expect(post.skipFirst(guild, pixivArtwork)).To(BeFalse())
	})
})

var _ = Describe("Limit Handler Tests", func() {
	var (
		post    Post
		artwork = []*discordgo.MessageSend{{Content: "1"}, {Content: "2"}, {Content: "3"}, {Content: "4"}}
	)

	bundles := func(sends ...[]*discordgo.MessageSend) []render.Bundle {
		out := make([]render.Bundle, 0, len(sends))
		for ind, s := range sends {
			out = append(out, render.Bundle{ID: strconv.Itoa(ind), Sends: s})
		}

		return out
	}

	It("should return the same array if limit not exceeded", func() {
		result := post.handleLimit(bundles(artwork), 4)
		Expect(result[0].Sends).To(HaveLen(4))
	})

	It("should cut over limit if one artwork", func() {
		result := post.handleLimit(bundles(artwork), 2)
		Expect(result[0].Sends).To(HaveLen(2))
		Expect(result[0].Sends[0].Content).To(
			Equal("Album size `(4)` exceeds the server's limit `(2)`, album has been cut."),
		)
	})

	It("should cut all except first page if two artworks", func() {
		result := post.handleLimit(bundles(artwork, artwork), 2)
		Expect(result).To(HaveLen(2))
		Expect(result[0].Sends).To(HaveLen(1))
		Expect(result[1].Sends).To(HaveLen(1))
		Expect(result[0].Sends[0].Content).To(
			Equal("Album size `(8)` exceeds the server's limit `(2)`, only the first image of every artwork has been sent."),
		)
	})

	It("should cut all except first page ignoring limit if more than one artwork", func() {
		result := post.handleLimit(bundles(artwork, artwork, artwork), 2)
		Expect(result).Should(HaveLen(3))
		Expect(result[0].Sends[0].Content).To(
			Equal("Album size `(12)` exceeds the server's limit `(2)`, only the first image of every artwork has been sent."),
		)
	})

	It("should keep each bundle ID with its first page", func() {
		result := post.handleLimit(bundles(artwork, artwork), 2)
		Expect(result[0].ID).To(Equal("0"))
		Expect(result[1].ID).To(Equal("1"))
	})
})

var _ = Describe("Skip Pages Tests", func() {
	var (
		post     Post
		artworks = []*discordgo.MessageSend{
			{Content: "1"}, {Content: "2"}, {Content: "3"}, {Content: "4"},
		}
	)

	It("should include first two", func() {
		post.Indices = map[int]struct{}{
			1: {}, 2: {},
		}

		post.SkipMode = SkipModeInclude
		result := post.skipArtworks(artworks)

		Expect(result).Should(And(
			HaveLen(2),
			ContainElements(&discordgo.MessageSend{Content: "1"}, &discordgo.MessageSend{Content: "2"}),
		))
	})

	It("should include last two", func() {
		post.Indices = map[int]struct{}{
			3: {}, 4: {},
		}

		post.SkipMode = SkipModeInclude
		result := post.skipArtworks(artworks)

		Expect(result).Should(And(
			HaveLen(2),
			ContainElements(&discordgo.MessageSend{Content: "3"}, &discordgo.MessageSend{Content: "4"}),
		))
	})

	It("should exclude last two", func() {
		post.Indices = map[int]struct{}{
			3: {}, 4: {},
		}

		post.SkipMode = SkipModeExclude
		result := post.skipArtworks(artworks)

		Expect(result).Should(And(
			HaveLen(2),
			ContainElements(&discordgo.MessageSend{Content: "1"}, &discordgo.MessageSend{Content: "2"}),
		))
	})

	It("shouldn't exclude anything", func() {
		post.Indices = map[int]struct{}{
			5: {},
		}

		post.SkipMode = SkipModeExclude
		result := post.skipArtworks(artworks)

		Expect(result).Should(And(
			HaveLen(4),
			ContainElements(
				&discordgo.MessageSend{Content: "1"}, &discordgo.MessageSend{Content: "2"},
				&discordgo.MessageSend{Content: "3"}, &discordgo.MessageSend{Content: "4"},
			),
		))
	})

	It("shouldn't include anything", func() {
		post.Indices = map[int]struct{}{
			5: {},
		}

		post.SkipMode = SkipModeInclude
		result := post.skipArtworks(artworks)

		Expect(result).Should(HaveLen(0))
	})
})
