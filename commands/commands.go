package commands

import (
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/router"
)

func RegisterCommands(b *bot.Bot) {
	b.Router.MustRegister(
		router.HelpCommand(router.HelpConfig{
			Name:        "help",
			Description: "Shows the list of commands or details about one",
			Category:    "General",
			Aliases:     []string{"documentation", "docs"},
			Title:       "Boe Tea Commands",
			Color:       0x439ef1,
			CategoryOrder: []string{
				"General", "Artworks", "Source", "Settings", "User", "Memes",
			},
		}),
	)

	for _, group := range [][]*router.Command{
		generalGroup(b),
		settingsGroup(b),
		userGroup(b),
		memesGroup(b),
		artworksGroup(b),
		ownerGroup(b),
		sourceGroup(b),
	} {
		b.Router.MustRegister(group...)
	}
}
