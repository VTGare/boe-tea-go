package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	Commands["set"] = Command{
		Name:            "set",
		Description:     "Show current guild settings or change them.",
		GuildOnly:       true,
		Exec:            set,
		Help:            true,
		AdvancedCommand: true,
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
				Value: "Amount of pictures considered a large set and procs a prompt. Must be an ***integer***. Set to 0 to ask every time",
			},
			{
				Name:  "pixiv",
				Value: "Whether to repost pixiv or not, accepts ***f or false (case-insensitive)*** to disable and ***t or true*** to enable.",
			},
			{
				Name:  "repost",
				Value: "Default reposting behaviour. Accepts ***links**, **embeds***, and ***ask*** options.",
			},
			{
				Name:  "reversesearch",
				Value: "Default reverse image search engine. Accepts ***SauceNAO*** or ***ASCII2D*** are available as of now.",
			},
			{
				Name:  "promptemoji",
				Value: "Confirmation prompt emoji. Only unicode or local server emoji's are allowed.",
			},
		},
	}
}

func set(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	switch len(args) {
	case 0:
		showGuildSettings(s, m)
	case 2:
		setting := args[0]
		newSetting := strings.ToLower(args[1])

		var err error
		var passedSetting interface{}
		switch setting {
		case "pixiv":
			passedSetting, err = strconv.ParseBool(newSetting)
		case "prefix":
			if unicode.IsLetter(rune(newSetting[len(newSetting)-1])) {
				passedSetting = newSetting + " "
			} else {
				passedSetting = newSetting
			}

			if len(passedSetting.(string)) > 5 {
				return errors.New("new prefix is too long")
			}
		case "largeset":
			passedSetting, err = strconv.Atoi(newSetting)
		case "repost":
			if newSetting != "ask" && newSetting != "embeds" && newSetting != "links" {
				return errors.New("unknown option. repost_as only accepts ``ask``, ``embeds``, and ``links`` options")
			}

			passedSetting = newSetting
		case "reversesearch":
			if newSetting != "saucenao" && newSetting != "ascii2d" {
				return errors.New("unknown option. repost_as only accepts ``ascii2d`` and ``saucenao`` options")
			}

			passedSetting = newSetting
		case "promptemoji":
			emoji, err := utils.GetEmoji(s, m.GuildID, newSetting)
			if err != nil {
				return errors.New("argument's either global emoji or not one at all")
			}
			passedSetting = emoji
		default:
			return errors.New("unknown setting " + setting)
		}

		if err != nil {
			return err
		}

		err = changeSetting(m.GuildID, setting, passedSetting)
		if err != nil {
			return err
		}
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Successfully changed ``%v`` to ``%v``", setting, newSetting))
	default:
		return errors.New("incorrect command usage. Please use help command for more information")
	}

	return nil
}

func showGuildSettings(s *discordgo.Session, m *discordgo.MessageCreate) {
	settings := database.GuildCache[m.GuildID]
	guild, _ := s.Guild(settings.GuildID)

	emoji := ""
	if utils.EmojiRegex.MatchString(settings.PromptEmoji) {
		emoji = settings.PromptEmoji
	} else {
		emoji = "<:" + settings.PromptEmoji + ">"
	}
	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:       "Current settings",
		Description: guild.Name,
		Color:       utils.EmbedColor,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Basic",
				Value: fmt.Sprintf("**Prefix:** %v", settings.Prefix),
			},
			{
				Name:  "Features",
				Value: fmt.Sprintf("**Pixiv:** %v\n**Reverse search:** %v", utils.FormatBool(settings.Pixiv), settings.ReverseSearch),
			},
			{
				Name:  "Pixiv settings",
				Value: fmt.Sprintf("**Large set**: %v\n**Repost**: %v\n**Prompt emoji**: %v", settings.LargeSet, settings.Repost, emoji),
			},
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: guild.IconURL(),
		},
		Timestamp: utils.EmbedTimestamp(),
	})
}

func changeSetting(guildID, setting string, newSetting interface{}) error {
	col := database.DB.Collection("guildsettings")

	res := col.FindOneAndUpdate(context.Background(), bson.M{
		"guild_id": guildID,
	}, bson.M{
		"$set": bson.M{
			setting: newSetting,
		},
	}, options.FindOneAndUpdate().SetReturnDocument(options.After))

	guild := &database.GuildSettings{}
	err := res.Decode(guild)
	if err != nil {
		return err
	}

	database.GuildCache[guildID] = *guild
	return nil
}
