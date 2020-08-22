package commands

import (
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
)

func init() {
	memes := CommandFramework.AddGroup("memes")
	memes.IsVisible = false

	memes.AddCommand("borgar", borgar)
	memes.AddCommand("brainpower", brainpower)
}

func borgar(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:     "🦕🍔",
		Timestamp: utils.EmbedTimestamp(),
		Color:     utils.EmbedColor,
		Image: &discordgo.MessageEmbedImage{
			URL: "https://images-ext-2.discordapp.net/external/gRgdT4gZIPbY26qK9iM0edWQA4hYPZF5RvxVdSeXhRQ/https/i.kym-cdn.com/photos/images/original/001/568/282/ef2.gif?width=438&height=444",
		},
	})
	return nil
}

func brainpower(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	s.ChannelMessageSend(m.ChannelID, "O-oooooooooo AAAAE-A-A-I-A-U- JO-oooooooooooo AAE-O-A-A-U-U-A- E-eee-ee-eee AAAAE-A-E-I-E-A-JO-ooo-oo-oo-oo EEEEO-A-AAA-AAAA")
	return nil
}
