package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
)

type settingFunc func(*discordgo.Session, *discordgo.MessageCreate, string) (interface{}, error)

var (
	settingMap = make(map[string]settingFunc)
)

func init() {
	settingMap["nsfw"] = setBool
	settingMap["pixiv"] = setBool
	settingMap["twitter"] = setBool
	settingMap["twitterprompt"] = setBool
	settingMap["crosspost"] = setBool
	settingMap["prefix"] = setPrefix
	settingMap["limit"] = setInt
	settingMap["repost"] = setRepost
}

func set(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	settings := database.GuildCache[m.GuildID]

	switch len(args) {
	case 0:
		showGuildSettings(s, m, settings)
	case 2:
		isAdmin, err := utils.MemberHasPermission(s, m.GuildID, m.Author.ID, discordgo.PermissionAdministrator)
		if err != nil {
			return err
		}
		if !isAdmin {
			return utils.ErrNoPermission
		}

		setting := args[0]
		newSetting := strings.ToLower(args[1])

		switch setting {
		case "prompt":
			setting = "twitterprompt"
		case "pixiv":
			s.ChannelMessageSend(m.ChannelID, "Please don't change this setting. Pixiv API is currently down and it's not in my power to fix it. Sorry for inconvenience...")
		}

		if new, ok := settingMap[setting]; ok {
			n, err := new(s, m, newSetting)
			if err != nil {
				return err
			}
			err = database.DB.ChangeSetting(m.GuildID, setting, n)
			if err != nil {
				return err
			}
			embed := &discordgo.MessageEmbed{
				Title: "✅ Successfully changed a setting!",
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:   "Setting",
						Value:  setting,
						Inline: true,
					},
					{
						Name:   "New value",
						Value:  newSetting,
						Inline: true,
					},
				},
				Color:     utils.EmbedColor,
				Timestamp: utils.EmbedTimestamp(),
			}
			s.ChannelMessageSendEmbed(m.ChannelID, embed)
		} else {
			return fmt.Errorf("invalid setting name: %v", setting)
		}
	default:
		return errors.New("incorrect command usage. Please use bt!help set command for more information")
	}

	return nil
}

func showGuildSettings(s *discordgo.Session, m *discordgo.MessageCreate, settings *database.GuildSettings) {
	guild, _ := s.Guild(settings.ID)

	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:       "Current settings",
		Description: guild.Name,
		Color:       utils.EmbedColor,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "General",
				Value: fmt.Sprintf("**Prefix:** %v | **NSFW:** %v", settings.Prefix, utils.FormatBool(settings.NSFW)),
			},
			{
				Name:  "Features",
				Value: fmt.Sprintf("**Repost:** %v | **Crosspost**: %v", settings.Repost, utils.FormatBool(settings.Crosspost)),
			},
			{
				Name:  "Pixiv settings",
				Value: fmt.Sprintf("**Auto-repost (pixiv)**: %v | **Limit**: %v", utils.FormatBool(settings.Pixiv), settings.Limit),
			},
			{
				Name:  "Twitter settings",
				Value: fmt.Sprintf("**Auto-repost (twitter)**: %v | **Prompt**: %v", utils.FormatBool(settings.Twitter), utils.FormatBool(settings.TwitterPrompt)),
			},
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: guild.IconURL(),
		},
		Timestamp: utils.EmbedTimestamp(),
	})
}

func setBool(s *discordgo.Session, m *discordgo.MessageCreate, str string) (interface{}, error) {
	return utils.ParseBool(str)
}

func setPrefix(s *discordgo.Session, m *discordgo.MessageCreate, str string) (interface{}, error) {
	if unicode.IsLetter(rune(str[len(str)-1])) {
		str += " "
	}
	if len(str) > 5 {
		return nil, fmt.Errorf("new prefix (%v) is too long (%v). Maximum length is %v", s, len(str), 5)
	}
	return str, nil
}

func setInt(s *discordgo.Session, m *discordgo.MessageCreate, str string) (interface{}, error) {
	ls, err := strconv.Atoi(str)
	if err != nil {
		return nil, utils.ErrParsingArgument
	}
	return ls, nil
}

func setRepost(s *discordgo.Session, m *discordgo.MessageCreate, str string) (interface{}, error) {
	if str != "disabled" && str != "enabled" && str != "strict" {
		return nil, errors.New("unknown option. repost only accepts enabled, disabled, and strict options")
	}

	if str == "enabled" || str == "strict" {
		description := "Repost checking requires collecting following data. Do you agree sharing this information?"
		if str == "strict" {
			description += "\nPlease enable Manage Messages permission to remove reposts with strict mode on, otherwise strict mode is useless."
		}

		agree := utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
			Embed: &discordgo.MessageEmbed{
				Title:     "Warning!",
				Color:     utils.EmbedColor,
				Timestamp: utils.EmbedTimestamp(),
				Thumbnail: &discordgo.MessageEmbedThumbnail{
					URL: utils.DefaultEmbedImage,
				},
				Description: description,
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:  "Post content",
						Value: "Pixiv ID or Twitter link. Essential for repost checking for obvious reasons",
					},
					{
						Name:  "Date and time of posting",
						Value: "Required to remove repost from a database in 24 hours",
					},
					{
						Name:  "Poster's username (without an ID or discriminator)",
						Value: "Required to give more information about the original poster when repost is detected",
					},
					{
						Name:  "Guild ID, message ID, and channel ID",
						Value: "Essential for repost checking. Required to find a repost in a database and create a link to the original post.",
					},
				},
			},
		})
		if agree {
			return str, nil
		}
		return nil, errors.New("cancelled enabling repost checker, ignore this error")
	}
	return str, nil
}
