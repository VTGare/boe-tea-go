package commands

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
)

func init() {
	basicGroup := CommandGroup{
		Name:        "basic",
		Description: "General purpose commands.",
		NSFW:        false,
		Commands:    make(map[string]Command),
		IsVisible:   true,
	}

	pingCommand := newCommand("ping", "Checks if Boe Tea is online and sends response time back.")
	pingCommand.setExec(ping)
	helpCommand := newCommand("help", "Sends this message. Use ``bt!help <group name> <command name>`` for more info about specific commands. ``bt!help <group>`` to list commands in a group.")
	helpCommand.setExec(help)
	feedbackCommand := newCommand("feedback", "Sends a feedback message to bot's author. Use ``bt!help basic feedback`` to see bugreport template.")
	feedbackCommand.setExec(feedback).setAliases("report").setHelp(&HelpSettings{
		IsVisible: true,
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
	})
	inviteCommand := newCommand("invite", "Sends a Boe Tea's invite link.").setExec(invite)
	setCommand := newCommand("set", "Show server's settings or change them.").setExec(set).setAliases("settings", "config", "cfg").setHelp(&HelpSettings{
		IsVisible: true,
		ExtendedHelp: []*discordgo.MessageEmbedField{
			{
				Name:  "Usage",
				Value: "bt!set ``<setting>`` ``<new setting>``",
			},
			{
				Name:  "prefix",
				Value: "Changes bot's prefix. Maximum ***5 characters***. If last character is a letter whitespace is assumed (takes one character).",
			},
			{
				Name:  "largeset",
				Value: "Amount of pictures considered a large set, which invokes a prompt. Must be an ***integer***. Set to 0 to ask every time",
			},
			{
				Name:  "limit",
				Value: "Image set size hard limit. If you attempt to repost a post or bulk post more than the limit it'll fail",
			},
			{
				Name:  "pixiv",
				Value: "Pixiv reposting switch, accepts ***f or false (case-insensitive)*** to disable and ***t or true*** to enable.",
			},
			{
				Name:  "repost",
				Value: "Repost check setting, accepts ***enabled***, ***disabled***, and ***strict*** settings. Strict mode disables a prompt and removes Twitter reposts (if bot has Manage Messages permission)",
			},
			{
				Name:  "reversesearch",
				Value: "Default reverse image search engine. Only ***SauceNAO*** or ***WAIT*** are available as of now.",
			},
			{
				Name:  "promptemoji",
				Value: "Confirmation prompt emoji. Only unicode or local server emoji's are allowed.",
			},
		},
	}).setGuildOnly(true)

	basicGroup.addCommand(pingCommand)
	basicGroup.addCommand(helpCommand)
	basicGroup.addCommand(feedbackCommand)
	basicGroup.addCommand(inviteCommand)
	basicGroup.addCommand(setCommand)
	CommandGroups["basic"] = basicGroup
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
		Description: "Use ``bt!help <group name> <command name>`` for extended help on specific commands.",
		Color:       utils.EmbedColor,
		Timestamp:   utils.EmbedTimestamp(),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: "https://i.imgur.com/OZ1Al5h.png",
		},
	}

	switch len(args) {
	case 0:
		embed.Title = "Help"
		for _, group := range CommandGroups {
			if group.IsVisible {
				field := &discordgo.MessageEmbedField{
					Name:  group.Name,
					Value: group.Description,
				}
				embed.Fields = append(embed.Fields, field)
			}
		}
	case 1:
		if group, ok := CommandGroups[args[0]]; ok {
			embed.Title = fmt.Sprintf("%v group command list", args[0])

			used := map[string]bool{}
			for _, command := range group.Commands {
				_, ok := used[command.Name]
				if command.Help.IsVisible && !ok {
					field := &discordgo.MessageEmbedField{
						Name:  command.Name,
						Value: command.createHelp(),
					}
					used[command.Name] = true
					embed.Fields = append(embed.Fields, field)
				}
			}
		} else {
			return fmt.Errorf("unknown group %v", args[0])
		}
	case 2:
		if group, ok := CommandGroups[args[0]]; ok {
			if command, ok := group.Commands[args[1]]; ok {
				if command.Help.IsVisible && command.Help.ExtendedHelp != nil {
					embed.Title = fmt.Sprintf("%v command extended help", command.Name)
					embed.Fields = command.Help.ExtendedHelp
				} else {
					return fmt.Errorf("command %v is invisible or doesn't have extended help", args[0])
				}
			} else {
				return fmt.Errorf("unknown command %v", args[1])
			}
		} else {
			return fmt.Errorf("unknown group %v", args[0])
		}
	default:
		return errors.New("incorrect command usage. Example: bt!help <group> <command name>")
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

func invite(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	_, err := s.ChannelMessageSend(m.ChannelID, "**Here's my invitation link, spread the word:** https://discordapp.com/api/oauth2/authorize?client_id=636468907049353216&permissions=537250880&scope=bot")
	if err != nil {
		return err
	}

	return nil
}
