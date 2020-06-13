package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
)

func init() {
	Commands["ping"] = Command{
		Name:            "ping",
		Description:     "Checks if Boe Tea is online and sends response time.",
		GuildOnly:       false,
		Exec:            ping,
		Help:            true,
		AdvancedCommand: false,
		ExtendedHelp:    nil,
	}
	Commands["help"] = Command{
		Name:            "help",
		Description:     "Sends this message. ``bt!help <command name>`` for extended help on other commands.",
		GuildOnly:       false,
		Exec:            help,
		Help:            true,
		AdvancedCommand: false,
		ExtendedHelp:    nil,
	}
	Commands["feedback"] = Command{
		Name:            "feedback",
		Description:     "Sends a feedback message to bot's author.",
		GuildOnly:       false,
		Exec:            feedback,
		Help:            true,
		AdvancedCommand: true,
		ExtendedHelp: []*discordgo.MessageEmbedField{
			{
				Name:  "Usage",
				Value: "``bt!feedback [feedback message]``. Please use this command to report bugs or suggest new features only. If you misuse this command you'll get blacklisted!",
			},
			{
				Name:  "feedback message",
				Value: "While suggestions can be plain text, bug reports are expected to be formatted in a specific way. Template shown below:\n```**Summary:** -\n**Reproduction:** -\n**Expected result:** -\n**Actual result:** -```\nYou can provide images as links or a single image as an attachment to the feedback message!",
			},
		},
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
		Description: "Use ``bt!help <command name>`` for extended help on specific commands.",
		Color:       utils.EmbedColor,
		Timestamp:   utils.EmbedTimestamp(),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: "https://i.imgur.com/OZ1Al5h.png",
		},
	}

	if len(args) == 0 {
		embed.Title = "Help"
		for _, command := range Commands {
			if command.Help {
				field := &discordgo.MessageEmbedField{
					Name:  command.Name,
					Value: command.Description,
				}
				embed.Fields = append(embed.Fields, field)
			}
		}
	} else {
		if command, ok := Commands[args[0]]; ok && command.AdvancedCommand && command.Help {
			embed.Fields = command.ExtendedHelp
			embed.Title = command.Name + " command help"
		} else {
			s.ChannelMessageSend(m.ChannelID, "The command either doesn't exist or has no extended help.")
			return nil
		}
	}

	s.ChannelMessageSendEmbed(m.ChannelID, embed)
	return nil
}

func feedback(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	message := strings.Join(args, " ")
	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("Feedback from %v", m.Author.String()),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: m.Author.AvatarURL(""),
		},
		Description: message,
		Timestamp:   utils.EmbedTimestamp(),
		Color:       utils.EmbedColor,
	}

	if len(m.Attachments) >= 1 {
		embed.Image = &discordgo.MessageEmbedImage{
			URL: m.Attachments[0].URL,
		}
	}

	ch, _ := s.UserChannelCreate(utils.AuthorID)
	_, err := s.ChannelMessageSendEmbed(ch.ID, embed)
	if err != nil {
		return err
	}

	return nil
}
