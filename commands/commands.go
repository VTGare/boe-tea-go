package commands

import "github.com/VTGare/boe-tea-go/bot"

func RegisterCommands(b *bot.Bot) {
	generalGroup(b)
	settingsGroup(b)
	userGroup(b)
	memesGroup(b)
	artworksGroup(b)
	ownerGroup(b)
	sourceGroup(b)
}
