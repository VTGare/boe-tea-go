package appcommands

import (
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/bwmarrin/discordgo"
)

type Command struct {
	ApplicationCommand *discordgo.ApplicationCommand
	Handler            HandlerFn
}

type HandlerFn func(s *discordgo.Session, i *discordgo.InteractionCreate)

var cmds = map[string]*Command{
	"ping": {
		ApplicationCommand: &discordgo.ApplicationCommand{
			Name:        "ping",
			Description: "Pong",
		},
		Handler: func(s *discordgo.Session, i *discordgo.InteractionCreate) {
			s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: &discordgo.InteractionResponseData{
					Content: "hello world",
				},
			})
		},
	},
}

func Register(b *bot.Bot) error {
	var (
		appID   = b.Config.Discord.ApplicationID
		guildID = b.Config.Discord.TestGuildID
	)

	for _, shard := range b.ShardManager.Shards {
		for _, cmd := range cmds {
			_, err := shard.Session.ApplicationCommandCreate(appID, guildID, cmd.ApplicationCommand)
			if err != nil {
				b.Log.With("error", err).Error("failed to create an application command")
			}
		}

	}

	return nil
}

func Handler(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if cmd, ok := cmds[i.ApplicationCommandData().Name]; ok {
		cmd.Handler(s, i)
	}
}
