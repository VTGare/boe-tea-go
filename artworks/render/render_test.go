package render

import (
	"time"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/bwmarrin/discordgo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func testRendered() artworks.Rendered {
	return artworks.Rendered{
		Title:       "Art by Author",
		URL:         "https://example.com/a",
		Timestamp:   time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC),
		Images:      []artworks.RenderedImage{{Preview: "https://example.com/1.png"}},
		Description: "a description",
		AIGenerated: false,
	}
}

func testOptions() Options {
	return Options{
		TagsEnabled: true,
		Reference: &discordgo.MessageReference{
			GuildID:   "g",
			ChannelID: "c",
			MessageID: "m",
		},
	}
}

var _ = Describe("Build pagination", func() {
	It("renders one page without a suffix for single images", func() {
		bundles := Build([]Input{{ID: "a", Rendered: testRendered()}}, testOptions())

		Expect(bundles).To(HaveLen(1))
		Expect(bundles[0].ID).To(Equal("a"))
		Expect(bundles[0].Sends).To(HaveLen(1))

		embed := bundles[0].Sends[0].Embeds[0]
		Expect(embed.Title).To(Equal("Art by Author"))
		Expect(embed.URL).To(Equal("https://example.com/a"))
		Expect(embed.Image.URL).To(Equal("https://example.com/1.png"))
		Expect(embed.Description).To(Equal("a description"))
	})

	It("suffixes titles and repeats footers across pages", func() {
		rendered := testRendered()
		rendered.Images = append(rendered.Images, artworks.RenderedImage{Preview: "https://example.com/2.png"})

		bundles := Build([]Input{{ID: "a", Footer: "quote", Rendered: rendered}}, testOptions())

		Expect(bundles[0].Sends).To(HaveLen(2))
		Expect(bundles[0].Sends[0].Embeds[0].Title).To(Equal("Art by Author | Page 1 / 2"))
		Expect(bundles[0].Sends[1].Embeds[0].Title).To(Equal("Art by Author | Page 2 / 2"))
		Expect(bundles[0].Sends[0].Embeds[0].Footer.Text).To(Equal("quote"))
		Expect(bundles[0].Sends[1].Embeds[0].Footer.Text).To(Equal("quote"))
	})

	It("renders imageless artwork as one page", func() {
		rendered := testRendered()
		rendered.Images = nil

		bundles := Build([]Input{{ID: "a", Rendered: rendered}}, testOptions())

		Expect(bundles).To(HaveLen(1))
		Expect(bundles[0].Sends).To(HaveLen(1))
		Expect(bundles[0].Sends[0].Embeds[0].Title).To(Equal("Art by Author"))
	})

	It("drops the first page when flagged", func() {
		rendered := testRendered()
		rendered.Images = append(rendered.Images, artworks.RenderedImage{Preview: "https://example.com/2.png"})

		bundles := Build([]Input{{ID: "a", Rendered: rendered, SkipFirstPage: true}}, testOptions())

		Expect(bundles[0].Sends).To(HaveLen(1))
		Expect(bundles[0].Sends[0].Embeds[0].Title).To(Equal("Art by Author | Page 2 / 2"))
	})

	It("attaches files to the first page only", func() {
		rendered := testRendered()
		rendered.Images = append(rendered.Images, artworks.RenderedImage{Preview: "https://example.com/2.png"})
		rendered.Files = []*discordgo.File{{Name: "v.mp4"}}

		bundles := Build([]Input{{ID: "a", Rendered: rendered}}, testOptions())

		Expect(bundles[0].Sends[0].Files).To(HaveLen(1))
		Expect(bundles[0].Sends[1].Files).To(BeEmpty())
	})
})

var _ = Describe("Build fields and tags", func() {
	fields := []artworks.RenderedField{
		{Name: "Likes", Value: "3", Inline: true},
	}

	It("keeps fields on the first page only by default", func() {
		rendered := testRendered()
		rendered.Images = append(rendered.Images, artworks.RenderedImage{Preview: "https://example.com/2.png"})
		rendered.Fields = fields

		bundles := Build([]Input{{ID: "a", Rendered: rendered}}, testOptions())

		Expect(bundles[0].Sends[0].Embeds[0].Fields).To(HaveLen(1))
		Expect(bundles[0].Sends[1].Embeds[0].Fields).To(BeEmpty())
	})

	It("adds an original-quality field per image with an original", func() {
		rendered := testRendered()
		rendered.Images = []artworks.RenderedImage{
			{Preview: "https://example.com/p1.png", Original: "https://example.com/o1.png"},
			{Preview: "https://example.com/p2.png", Original: "https://example.com/o2.png"},
			{Preview: "https://example.com/p3.png"},
		}
		rendered.Fields = fields

		bundles := Build([]Input{{ID: "a", Rendered: rendered}}, testOptions())

		first := bundles[0].Sends[0].Embeds[0].Fields
		second := bundles[0].Sends[1].Embeds[0].Fields
		third := bundles[0].Sends[2].Embeds[0].Fields

		Expect(first).To(HaveLen(2))
		Expect(first[0].Name).To(Equal("Likes"))
		Expect(first[1].Name).To(Equal("Original quality"))
		Expect(first[1].Value).To(Equal("[Click here](https://example.com/o1.png)"))

		Expect(second).To(HaveLen(1))
		Expect(second[0].Name).To(Equal("Original quality"))
		Expect(second[0].Value).To(Equal("[Click here](https://example.com/o2.png)"))

		Expect(third).To(BeEmpty())
	})

	It("renders linked tags under the standard header", func() {
		rendered := testRendered()
		rendered.Description = ""
		rendered.Tags = []string{"tag1", "tag2"}
		rendered.TagLinkTemplate = "[%v](https://example.com/tags/%v)"

		bundles := Build([]Input{{ID: "a", Rendered: rendered}}, testOptions())

		Expect(bundles[0].Sends[0].Embeds[0].Description).To(Equal(
			"**Tags:**\n[tag1](https://example.com/tags/tag1) • [tag2](https://example.com/tags/tag2)",
		))
	})

	It("renders plain tags under the standard header after text", func() {
		rendered := testRendered()
		rendered.Tags = []string{"tag1", "tag2"}

		bundles := Build([]Input{{ID: "a", Rendered: rendered}}, testOptions())

		Expect(bundles[0].Sends[0].Embeds[0].Description).To(Equal(
			"a description\n\n**Tags:**\ntag1 • tag2",
		))
	})

	It("starts the tag block without blank lines on empty text", func() {
		rendered := testRendered()
		rendered.Description = ""
		rendered.Tags = []string{"tag1"}
		rendered.TagLinkTemplate = "[%v](https://bsky.app/hashtag/%v)"

		bundles := Build([]Input{{ID: "a", Rendered: rendered}}, testOptions())

		Expect(bundles[0].Sends[0].Embeds[0].Description).To(Equal(
			"**Tags:**\n[tag1](https://bsky.app/hashtag/tag1)",
		))
	})

	It("ignores tags when disabled", func() {
		rendered := testRendered()
		rendered.Tags = []string{"tag1"}

		opts := testOptions()
		opts.TagsEnabled = false

		bundles := Build([]Input{{ID: "a", Rendered: rendered}}, opts)

		Expect(bundles[0].Sends[0].Embeds[0].Description).To(Equal("a description"))
	})

	It("adds the AI disclaimer when flagged", func() {
		rendered := testRendered()
		rendered.AIGenerated = true

		bundles := Build([]Input{{ID: "a", Rendered: rendered}}, testOptions())

		fields := bundles[0].Sends[0].Embeds[0].Fields
		Expect(fields).To(HaveLen(1))
		Expect(fields[0].Name).To(Equal("⚠️ Disclaimer"))
		Expect(fields[0].Value).To(Equal("This artwork is AI-generated."))
	})
})

var _ = Describe("Build correlation and decoration", func() {
	It("keeps each ID with its own pages", func() {
		inputs := []Input{
			{ID: "a", Rendered: testRendered()},
			{ID: "b", Rendered: testRendered()},
		}

		bundles := Build(inputs, testOptions())

		Expect(bundles).To(HaveLen(2))
		Expect(bundles[0].ID).To(Equal("a"))
		Expect(bundles[1].ID).To(Equal("b"))
	})

	It("references the triggering message on every page", func() {
		rendered := testRendered()
		rendered.Images = append(rendered.Images, artworks.RenderedImage{Preview: "https://example.com/2.png"})

		bundles := Build([]Input{{ID: "a", Rendered: rendered}}, testOptions())

		for _, send := range bundles[0].Sends {
			Expect(send.Reference.MessageID).To(Equal("m"))
			Expect(send.AllowedMentions).NotTo(BeNil())
			Expect(send.Embeds[0].Author).To(BeNil())
		}
	})

	It("stamps the crosspost author instead of the reference", func() {
		opts := testOptions()
		opts.Crosspost = true
		opts.AuthorName = "posted by tester"
		opts.AuthorIconURL = "https://example.com/icon.png"

		bundles := Build([]Input{{ID: "a", Rendered: testRendered()}}, opts)

		embed := bundles[0].Sends[0].Embeds[0]
		Expect(embed.Author.Name).To(Equal("posted by tester"))
		Expect(bundles[0].Sends[0].Reference).To(BeNil())
	})
})
