package commands

import "github.com/VTGare/boe-tea-go/pkg/bot"

func RegisterCommands(b *bot.Bot) {
	generalGroup(b)
	userGroup(b)
	memesGroup(b)
	artworksGroup(b)
	ownerGroup(b)
	sourceGroup(b)
}
