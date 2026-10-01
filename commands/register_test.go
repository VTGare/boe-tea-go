package commands

import (
	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/artworks/bluesky"
	"github.com/VTGare/boe-tea-go/artworks/deviant"
	"github.com/VTGare/boe-tea-go/artworks/pixiv"
	"github.com/VTGare/boe-tea-go/artworks/twitter"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/router"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// realProviders are the providers main registers.
func realProviders() []artworks.Provider {
	return []artworks.Provider{twitter.New(), deviant.New(), bluesky.New(), pixiv.New("")}
}

var _ = Describe("RegisterCommands", func() {
	It("registers every command without validation errors", func() {
		b := &bot.Bot{Router: router.New(router.Config{}), ArtworkProviders: realProviders()}

		Expect(func() { RegisterCommands(b) }).NotTo(Panic())
		Expect(b.Router.Commands()).NotTo(BeEmpty())
	})

	It("gives every provider a /set toggle under its key and aliases", func() {
		b := &bot.Bot{Router: router.New(router.Config{}), ArtworkProviders: realProviders()}
		RegisterCommands(b)

		for _, p := range b.ArtworkProviders {
			info := p.Info()
			Expect(b.Router.Lookup("set", info.Key)).NotTo(BeNil(), info.Key)

			for _, alias := range info.Aliases {
				Expect(b.Router.Lookup("set", alias)).To(BeIdenticalTo(b.Router.Lookup("set", info.Key)))
			}
		}
	})

	It("keeps the old prefix names of merged commands", func() {
		b := &bot.Bot{Router: router.New(router.Config{}), ArtworkProviders: realProviders()}
		RegisterCommands(b)

		for _, name := range []string{
			"share", "si", "shareexclude", "ex", "crosspostexclude", "cp",
			"unbookmark", "unfav", "userset", "ls",
		} {
			Expect(b.Router.Lookup(name)).NotTo(BeNil(), name)
		}
	})

	It("registers only the merged slash commands", func() {
		b := &bot.Bot{Router: router.New(router.Config{}), ArtworkProviders: realProviders()}
		RegisterCommands(b)

		global, _ := b.Router.ApplicationCommands()
		names := make([]string, 0, len(global))
		for _, c := range global {
			names = append(names, c.Name)
		}

		Expect(names).To(ContainElements("share", "groups", "bookmarks", "profile", "Find Sauce", "Find Sauce (Private)"))
		for _, old := range []string{"shareexclude", "crosspostexclude", "newgroup", "unbookmark", "userset"} {
			Expect(names).NotTo(ContainElement(old))
		}
	})
})
