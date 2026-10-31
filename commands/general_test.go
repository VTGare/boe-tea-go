package commands

import (
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/stats"
	"github.com/VTGare/gumi/v2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("commandStatsEmbed", func() {
	It("fits every registered command within Discord's embed limits", func() {
		b := &bot.Bot{Router: gumi.New(gumi.Config{}), ArtworkProviders: realProviders()}
		RegisterCommands(b)

		s := stats.New(b.Router, b.ArtworkProviders)
		items, _ := s.CommandStats()
		for i := range items {
			items[i].Count = 9_999_999
		}

		Expect(len(items)).To(BeNumerically(">", 25))

		embed := commandStatsEmbed(items)
		Expect(len(embed.Fields)).To(BeNumerically("<=", 25))
		Expect(len(embed.Description)).To(BeNumerically("<=", 4096))

		for _, item := range items {
			Expect(embed.Description).To(ContainSubstring(item.Name))
		}
	})

	It("lists the counts in the given order", func() {
		embed := commandStatsEmbed([]stats.Item{
			{Name: "share", Count: 2311},
			{Name: "Find Sauce", Count: 23},
		})

		Expect(embed.Description).To(Equal("```\nshare       2311\nFind Sauce  23\n```"))
	})
})
