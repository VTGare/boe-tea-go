package main

import (
	"fmt"
	"strings"

	"github.com/VTGare/boe-tea-go/commands"
	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/pixivhelper"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	botMention      string
	defaultPrefixes = []string{"bt!", "bt.", "bt "}
)

func onReady(s *discordgo.Session, e *discordgo.Ready) {
	botMention = "<@!" + e.User.ID + ">"
	log.Infoln(e.User.String(), "is ready.")

	err := utils.CreateDB(e.Guilds)
	if err != nil {
		log.Warnln("Error adding guilds: ", err)
	}
}

func trimPrefix(content, guildID string) string {
	guild, ok := database.GuildCache[guildID]
	var defaultPrefix bool
	if ok && guild.Prefix == "bt!" {
		defaultPrefix = true
	} else if !ok {
		defaultPrefix = true
	} else {
		defaultPrefix = false
	}

	switch {
	case strings.HasPrefix(content, botMention):
		return strings.TrimPrefix(content, botMention)
	case defaultPrefix:
		for _, prefix := range defaultPrefixes {
			if strings.HasPrefix(content, prefix) {
				return strings.TrimPrefix(content, prefix)
			}
		}
	case !defaultPrefix && ok:
		return strings.TrimPrefix(content, guild.Prefix)
	default:
		return content
	}

	return content
}

func handleError(s *discordgo.Session, m *discordgo.MessageCreate, err error) {
	if err != nil {
		log.Errorf("An error occured: %v", err)
		embed := &discordgo.MessageEmbed{
			Title: "Oops, something went wrong!",
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: "https://i.imgur.com/OZ1Al5h.png",
			},
			Description: fmt.Sprintf(`***Error message:***
			%v

			Please contact bot's author using bt!feedback command or directly at VTGare#3370 if you can't understand the error. 
			`, err),
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
		}
		s.ChannelMessageSendEmbed(m.ChannelID, embed)
	}
}

func prefixless(s *discordgo.Session, m *discordgo.MessageCreate) error {
	guild := database.GuildCache[m.GuildID]
	if guild.Repost == "strict" {
		ips := utils.RemoveReposts(s, m)
		if len(ips) != 0 {
			s.ChannelMessageSendEmbed(m.ChannelID, utils.RepostsToEmbed(ips...))
		}
	}

	if guild.Pixiv {
		matches := utils.PixivRegex.FindAllStringSubmatch(m.Content, len(m.Content)+1)
		if matches != nil {
			ids := make([]string, 0)
			for _, match := range matches {
				ids = append(ids, match[1])
			}

			log.Infof("Executing pixiv reposting. Guild ID: %v, channel ID: %v", m.GuildID, m.ChannelID)
			err := pixivhelper.PostPixiv(s, m, ids)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func messageCreated(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	isCommand := commands.CommandFramework.Handle(s, m)
	if !isCommand {
		err := prefixless(s, m)
		commands.CommandFramework.ErrorHandler(err)
	}
}

func reactCreated(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if author, ok := pixivhelper.EmbedCache[r.MessageID]; ok && author == r.UserID && r.Emoji.APIName() == "❌" {
		s.ChannelMessageDelete(r.ChannelID, r.MessageID)
	}
}

func guildCreated(s *discordgo.Session, g *discordgo.GuildCreate) {
	if len(database.GuildCache) == 0 {
		return
	}

	if _, ok := database.GuildCache[g.ID]; !ok {
		newGuild := database.DefaultGuildSettings(g.ID)
		err := database.InsertOneGuild(newGuild)
		if err != nil {
			log.Println(err)
		}

		database.GuildCache[g.ID] = *newGuild
		log.Infoln("Joined ", g.Name)
	}
}

func guildDeleted(s *discordgo.Session, g *discordgo.GuildDelete) {
	err := database.RemoveGuild(g.ID)
	if err != nil {
		log.Println(err)
	}

	delete(database.GuildCache, g.ID)
	log.Infoln("Kicked or banned from", g.Guild.Name, g.ID)
}
