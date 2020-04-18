package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"
	"unicode"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	Commands["set"] = Command{
		Name:        "set",
		Description: "Show current guild settings or change them.",
		GuildOnly:   true,
		Exec:        set,
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
		if setting == "pixiv" || setting == "twitter" {
			passedSetting, err = strconv.ParseBool(newSetting)
		} else if setting == "prefix" && unicode.IsLetter(rune(newSetting[len(newSetting)-1])) {
			passedSetting = newSetting + " "
		} else if setting == "largeset" {
			setting = "large_set"
			passedSetting, err = strconv.Atoi(newSetting)
		} else {
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
		Color:       0x439ef1,
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
		},
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: guild.IconURL(),
		},
		Timestamp: time.Now().Format(time.RFC3339),
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
