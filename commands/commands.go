package commands

import "github.com/VTGare/boe-tea-go/bot"

func Register(b *bot.Bot) {
	generalGroup(b)
	userGroup(b)
	memesGroup(b)
	artworksGroup(b)
	ownerGroup(b)
	sourceGroup(b)
}
