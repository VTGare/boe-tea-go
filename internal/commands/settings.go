package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/embeds"
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
	settingMap["reactions"] = setBool
	settingMap["twitterprompt"] = setBool
	settingMap["crosspost"] = setBool
	settingMap["prefix"] = setPrefix
	settingMap["limit"] = setInt
	settingMap["repost"] = setRepost
}

func set(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	settings := database.GuildCache[m.GuildID]
	eb := embeds.NewBuilder()

	switch {
	case len(args) == 0:
		showGuildSettings(s, m, settings)
	case len(args) >= 2:
		var (
			isAdmin, err = utils.MemberHasPermission(s, m.GuildID, m.Author.ID, discordgo.PermissionAdministrator)
		)

		if err != nil {
			return err
		}

		if !isAdmin {
			s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(utils.ErrNoPermission.Error()).Finalize())
			return nil
		}

		setting := args[0]
		newSetting := strings.ToLower(args[1])

		switch setting {
		case "prompt":
			setting = "twitterprompt"
		}

		if new, ok := settingMap[setting]; ok {
			n, err := new(s, m, newSetting)
			if err != nil {
				return err
			}

			if n != nil {
				err = database.DB.ChangeSetting(m.GuildID, setting, n)
				if err != nil {
					return err
				}
				eb.SuccessTemplate("Successfully changed a setting!").AddField("Setting", setting, true).AddField("New value", newSetting, true)
				s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
			}
		} else {
			eb.FailureTemplate(fmt.Sprintf("Setting [%v] doesn't exist", setting))
			s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
			return nil
		}
	default:
		eb.FailureTemplate(fmt.Sprintf("``bt!set`` requires either 2 arguments or no arguments."))
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	return nil
}

func showGuildSettings(s *discordgo.Session, m *discordgo.MessageCreate, settings *database.GuildSettings) {
	guild, _ := s.Guild(settings.ID)

	eb := embeds.NewBuilder().Title("Current settings").Description(guild.Name).Thumbnail(guild.IconURL())
	eb.AddField("General", fmt.Sprintf("**Prefix:** %v | **NSFW:** %v", settings.Prefix, utils.FormatBool(settings.NSFW)))
	eb.AddField("Features", fmt.Sprintf("**Repost:** %v | **Crosspost**: %v | **Auto-react (reactions):** %v", settings.Repost, utils.FormatBool(settings.Crosspost), utils.FormatBool(settings.Reactions)))
	eb.AddField("Pixiv settings", fmt.Sprintf("**Auto-repost (pixiv)**: %v | **Limit**: %v", utils.FormatBool(settings.Pixiv), settings.Limit))
	eb.AddField("Twitter settings", fmt.Sprintf("**Auto-repost (twitter)**: %v | **Prompt**: %v", utils.FormatBool(settings.Twitter), utils.FormatBool(settings.TwitterPrompt)))

	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
}

func setBool(_ *discordgo.Session, _ *discordgo.MessageCreate, str string) (interface{}, error) {
	return utils.ParseBool(str)
}

func setPrefix(s *discordgo.Session, _ *discordgo.MessageCreate, str string) (interface{}, error) {
	if unicode.IsLetter(rune(str[len(str)-1])) {
		str += " "
	}
	if len(str) > 5 {
		return nil, fmt.Errorf("new prefix (%v) is too long (%v). Maximum length is %v", s, len(str), 5)
	}
	return str, nil
}

func setInt(_ *discordgo.Session, _ *discordgo.MessageCreate, str string) (interface{}, error) {
	ls, err := strconv.Atoi(str)
	if err != nil {
		return nil, utils.ErrParsingArgument
	}
	return ls, nil
}

func setString(_ *discordgo.Session, _ *discordgo.MessageCreate, str string) (interface{}, error) {
	return str, nil
}

func setRepost(s *discordgo.Session, m *discordgo.MessageCreate, str string) (interface{}, error) {
	eb := embeds.NewBuilder()
	if str != "disabled" && str != "enabled" && str != "strict" {
		return nil, errors.New("unknown option. repost only accepts enabled, disabled, and strict options")
	}

	if str == "enabled" || str == "strict" {
		description := "Repost checking requires collecting following data. Do you agree sharing this information?"
		if str == "strict" {
			description += "\nPlease enable Manage Messages permission to remove reposts with strict mode on, otherwise strict mode is useless."
		}

		eb.WarnTemplate(description).Thumbnail(utils.DefaultEmbedImage)
		eb.AddField("Artwork ID", "Pixiv ID or Twitter snowflake. Essential for repost checking!")
		eb.AddField("Timestamp", "Required to remove the repost from our database in 24 hours.")
		eb.AddField("Username", "Required to give more information about OP when repost is detected.")
		eb.AddField("Discord IDs", "Required to find an original post and create a link to it afterwards.")
		agree := utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
			Embed: eb.Finalize(),
		})
		if agree {
			return str, nil
		}

		s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate("Cancelled enabling repost module.").Finalize())
		return nil, nil
	}
	return str, nil
}
