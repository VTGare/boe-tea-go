package commands

import (
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/disgoorg/disgo/discord"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Settings toggles", func() {
	It("have unique names and aliases", func() {
		seen := make(map[string]bool)
		for _, t := range newSettingsSpec(realProviders()).toggles {
			for _, key := range append([]string{t.name}, t.aliases...) {
				Expect(seen).NotTo(HaveKey(key))
				seen[key] = true
			}
			Expect(t.section).To(BeElementOf("posting", "sources"))
		}
	})

	It("add one source toggle per provider, in registration order", func() {
		spec := newSettingsSpec(realProviders())

		var names []string
		for _, t := range spec.in("sources") {
			names = append(names, t.name)
		}

		Expect(names).To(Equal([]string{"twitter", "deviantart", "bluesky", "pixiv"}))
		Expect(spec.sourceLabels()).To(Equal("Twitter, DeviantArt, Bluesky, Pixiv"))
	})

	It("turn providers off and on by key", func() {
		spec := newSettingsSpec(realProviders())

		for _, p := range realProviders() {
			key := p.Info().Key
			t, ok := spec.find(key)
			Expect(ok).To(BeTrue())

			g := store.DefaultGuild("g")
			applyToggle(g, t, false)
			Expect(g.ProviderEnabled(key)).To(BeFalse())
			Expect(g.DisabledProviders).To(Equal([]string{key}))
			applyToggle(g, t, true)
			Expect(g.ProviderEnabled(key)).To(BeTrue())
		}
	})
})

var _ = Describe("Applying settings", func() {
	var g *store.Guild

	BeforeEach(func() { g = store.DefaultGuild("g") })

	DescribeTable("prefixes",
		func(in, want string) {
			c, err := applyPrefix(g, in)
			Expect(err).NotTo(HaveOccurred())
			Expect(g.Prefix).To(Equal(want))
			Expect(c.new).To(Equal("`" + want + "`"))
		},
		Entry("symbol", "!", "!"),
		Entry("trailing letter gets a space", "uwu", "uwu "),
		Entry("trimmed", "  ?  ", "?"),
	)

	DescribeTable("rejects bad prefixes",
		func(in string) {
			_, err := applyPrefix(g, in)
			Expect(err).To(HaveOccurred())
			Expect(g.Prefix).To(Equal("bt!"))
		},
		Entry("empty", "   "),
		Entry("too long", "toolong"),
		Entry("too long with the added space", "abcde"),
	)

	It("bounds the post limit", func() {
		c, err := applyLimit(g, 20)
		Expect(err).NotTo(HaveOccurred())
		Expect(c).To(Equal(settingChange{name: "Images per post", old: "10", new: "20", hint: "Longer galleries are cut off after this many"}))

		_, err = applyLimit(g, 0)
		Expect(err).To(HaveOccurred())
		_, err = applyLimit(g, 101)
		Expect(err).To(HaveOccurred())
		Expect(g.Posting.Limit).To(Equal(20))
	})

	It("changes repost mode and memory independently", func() {
		c, err := applyReposts(g, "strict", "")
		Expect(err).NotTo(HaveOccurred())
		Expect(g.Repost).To(Equal(store.Repost{Mode: store.RepostStrict, TTL: 24 * time.Hour}))
		Expect(c.old).To(Equal("Notify, 1 day"))
		Expect(c.new).To(Equal("Strict, 1 day"))

		_, err = applyReposts(g, "", "3d")
		Expect(err).NotTo(HaveOccurred())
		Expect(g.Repost.TTL).To(Equal(72 * time.Hour))

		_, err = applyReposts(g, "enabled", "")
		Expect(err).To(HaveOccurred())
		_, err = applyReposts(g, "", "2d")
		Expect(err).To(HaveOccurred())
	})

	DescribeTable("formats repost memory",
		func(d time.Duration, want string) { Expect(formatTTL(d)).To(Equal(want)) },
		Entry("one day", 24*time.Hour, "1 day"),
		Entry("days", 72*time.Hour, "3 days"),
		Entry("hours", 6*time.Hour, "6 hours"),
		Entry("minutes", 30*time.Minute, "30 minutes"),
		Entry("odd values", 90*time.Second, "1m30s"),
	)
})

var _ = Describe("Settings panel", func() {
	view := func(g *store.Guild) *panelView {
		spec := newSettingsSpec(realProviders())
		cmd := settingsGroup(&bot.Bot{ArtworkProviders: realProviders()})[0]
		return &panelView{cmd: cmd, spec: spec, owner: "123456789012345678", guild: g, name: "Server"}
	}

	DescribeTable("fits Discord's component limits",
		func(section string) {
			g := store.DefaultGuild("g")
			for i := range 200 {
				g.ArtChannels = append(g.ArtChannels, strconv.Itoa(1000000000000000000+i))
			}

			resp := view(g).render(section)
			Expect(resp.Embeds).To(HaveLen(1))
			Expect(len(resp.Embeds[0].Description)).To(BeNumerically("<=", 4096))
			Expect(len(resp.Components)).To(BeNumerically("<=", 5))

			for _, row := range resp.Components {
				comps := row.(discord.ActionRowComponent).Components
				Expect(comps).NotTo(BeEmpty())
				Expect(len(comps)).To(BeNumerically("<=", 5))

				for _, c := range comps {
					switch c := c.(type) {
					case discord.ButtonComponent:
						Expect(c.CustomID).To(HavePrefix("rt:settings:123456789012345678:"))
						Expect(len(c.CustomID)).To(BeNumerically("<=", 100))
						Expect(len(c.Label)).To(BeNumerically("<=", 80))
					case discord.StringSelectMenuComponent:
						Expect(comps).To(HaveLen(1))
						Expect(len(c.Options)).To(BeNumerically("<=", 25))
						Expect(c.CustomID).To(HavePrefix("rt:settings:"))
					case discord.ChannelSelectMenuComponent:
						Expect(comps).To(HaveLen(1))
						Expect(c.CustomID).To(HavePrefix("rt:settings:"))
					default:
						Fail("unexpected component")
					}
				}
			}
		},
		Entry("home", "home"),
		Entry("general", "general"),
		Entry("posting", "posting"),
		Entry("sources", "sources"),
		Entry("reposts", "reposts"),
		Entry("channels", "channels"),
	)

	It("shows toggle state in labels and styles", func() {
		g := store.DefaultGuild("g")
		g.SetProvider("pixiv", false)

		row := view(g).render("sources").Components[0].(discord.ActionRowComponent).Components
		twitter := row[0].(discord.ButtonComponent)
		pixiv := row[3].(discord.ButtonComponent)

		Expect(pixiv.Label).To(Equal("Pixiv: Off"))
		Expect(pixiv.Style).To(Equal(discord.ButtonStyleSecondary))
		Expect(pixiv.CustomID).To(HaveSuffix(":toggle:pixiv"))
		Expect(twitter.Label).To(Equal("Twitter: On"))
		Expect(twitter.Style).To(Equal(discord.ButtonStyleSuccess))
	})

	It("summarises every section on the home page", func() {
		g := store.DefaultGuild("g")
		g.Repost.Mode = store.RepostOff

		embed := view(g).render("home").Embeds[0]
		names := make([]string, 0, len(embed.Fields))
		for _, f := range embed.Fields {
			names = append(names, f.Name)
		}

		Expect(names).To(Equal([]string{"General", "Posting", "Sources", "Reposts", "Art channels · 0"}))
		Expect(embed.Fields[3].Value).To(HavePrefix("**Off**"))
		Expect(embed.Fields[4].Value).To(ContainSubstring("every channel"))
	})

	It("marks the current repost choices as selected", func() {
		resp := view(store.DefaultGuild("g")).render("reposts")

		mode := resp.Components[0].(discord.ActionRowComponent).Components[0].(discord.StringSelectMenuComponent)
		for _, o := range mode.Options {
			Expect(o.Default).To(Equal(o.Value == "notify"))
		}
	})
})

var _ = Describe("Art channels", func() {
	all := []channel{
		{ID: "cat", Type: discord.ChannelTypeGuildCategory},
		{ID: "art", Type: discord.ChannelTypeGuildText, ParentID: "cat"},
		{ID: "news", Type: discord.ChannelTypeGuildNews, ParentID: "cat"},
		{ID: "voice", Type: discord.ChannelTypeGuildVoice, ParentID: "cat"},
		{ID: "general", Type: discord.ChannelTypeGuildText},
	}

	It("expands categories, drops unpostable channels and duplicates", func() {
		picked := pickedChannels(all, []string{"general", "cat", "art", "voice"})
		Expect(expandArtChannels(all, picked)).To(Equal([]string{"general", "art", "news"}))
	})

	It("keeps unknown IDs so deleted channels can be removed", func() {
		Expect(expandArtChannels(all, pickedChannels(all, []string{"gone"}))).To(Equal([]string{"gone"}))
	})

	It("pages long lists and reports cleanups", func() {
		ids := make([]string, 45)
		for i := range ids {
			ids[i] = strconv.Itoa(i)
		}

		pages := channelPages(ids, 2)
		Expect(pages).To(HaveLen(3))
		Expect(pages[0].Footer.Text).To(Equal("Page 1 of 3 · Removed 2 deleted channels"))
		Expect(strings.Count(pages[2].Description, "<#")).To(Equal(5))

		empty := channelPages(nil, 0)
		Expect(empty).To(HaveLen(1))
		Expect(empty[0].Footer).To(BeNil())
	})

	It("summarises changes with skipped channels", func() {
		e := channelsChangedEmbed(true, []string{"a"}, []string{"b"})
		Expect(e.Description).To(Equal("**Added 1 art channel**\n<#a>\n-# <#b> already art channels"))
	})
})
