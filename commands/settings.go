package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"unicode"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	Commands["set"] = Command{
		Name:         "set",
		Description:  "Show current guild settings or change them.",
		GuildOnly:    true,
		Exec:         set,
		GroupCommand: true,
		ExtendedHelp: []*discordgo.MessageEmbedField{
			{
				Name:  "prefix",
				Value: "Changes server's prefix. Maximum 5 characters. If last character is a letter whitespace is assumed (takes one character).",
			},
			{
				Name:  "largeset",
				Value: "Amount of pictures considered a large set and procs a prompt. Must be an integer. Set to 0 to ask every time",
			},
			{
				Name:  "pixiv",
				Value: "Whether to repost pixiv or not, accepts [0, F, f, false, False, FALSE] as false and [1, T, t, true, True, TRUE] as true.",
			},
			{
				Name:  "twitter",
				Value: "Whether to repost twitter or not, accepts [0, F, f, false, False, FALSE] as false and [1, T, t, true, True, TRUE] as true.",
			},
			{
				Name:  "repost_as",
				Value: "Default behaviour when reposting images. Accepts **links**, **embeds**, and **ask** options.",
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
		newSetting := args[1]

		var err error
		var passedSetting interface{}
		switch setting {
		case "pixiv":
			passedSetting, err = strconv.ParseBool(newSetting)
		case "twitter":
			passedSetting, err = strconv.ParseBool(newSetting)
		case "prefix":
			if unicode.IsLetter(rune(newSetting[len(newSetting)-1])) {
				passedSetting = newSetting + " "
			} else {
				passedSetting = newSetting
			}
		case "largeset":
			setting = "large_set"
			passedSetting, err = strconv.Atoi(newSetting)
		case "repost_as":
			if newSetting != "ask" && newSetting != "embeds" && newSetting != "links" {
				return errors.New("unknown option. repost_as only takes ``ask embeds links`` options")
			}

			passedSetting = newSetting
		default:
			return errors.New("unknown setting ``" + setting + "``")
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
	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:       "Current settings",
		Description: guild.Name,
		Color:       utils.EmbedColor,
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Prefix",
				Value: settings.Prefix,
			},
			{
				Name:  "Large set",
				Value: strconv.Itoa(settings.LargeSet),
			},
			{
				Name:  "Pixiv",
				Value: strconv.FormatBool(settings.Pixiv),
			},
			{
				Name:  "Twitter",
				Value: strconv.FormatBool(settings.Twitter),
			},
			{
				Name:  "Repost as",
				Value: settings.RepostAs,
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
