package commands

import (
	"fmt"
	"time"

	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
)

func init() {
	Commands["ping"] = Command{
		Name:         "ping",
		Description:  "Checks if Boe Tea is online and sends response time.",
		GuildOnly:    false,
		Exec:         ping,
		GroupCommand: false,
		ExtendedHelp: nil,
	}
	Commands["help"] = Command{
		Name:         "help",
		Description:  "Sends Boe Tea's command documentation",
		GuildOnly:    false,
		Exec:         help,
		GroupCommand: false,
		ExtendedHelp: nil,
	}
}

func ping(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(":ping_pong: Pong! Latency: ***%v***", s.HeartbeatLatency().Round(1*time.Millisecond)))
	if err != nil {
		return err
	}
	return nil
}

func help(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	embed := &discordgo.MessageEmbed{
		Title:       "Help",
		Description: "Boe Tea's command documentation",
		Color:       utils.EmbedColor,
		Timestamp:   utils.EmbedTimestamp(),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: "https://i.imgur.com/OZ1Al5h.png",
		},
	}

	if len(args) == 0 {
		for _, command := range Commands {
			field := &discordgo.MessageEmbedField{
				Name:  command.Name,
				Value: command.Description,
			}
			embed.Fields = append(embed.Fields, field)
		}
	} else {
		if command, ok := Commands[args[0]]; ok && command.GroupCommand {
			embed.Fields = command.ExtendedHelp
		} else {
			s.ChannelMessageSend(m.ChannelID, "The command either doesn't exist or has no extended help.")
			return nil
		}
	}

	s.ChannelMessageSendEmbed(m.ChannelID, embed)
	return nil
}
