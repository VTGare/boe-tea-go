package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/VTGare/boe-tea-go/commands"
	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
)

var (
	botMention   string
	globalPrefix = "bt!"
)

func onReady(s *discordgo.Session, e *discordgo.Ready) {
	botMention = "<@!" + e.User.ID + ">"
	log.Println(e.User.String(), "is ready.")

	err := utils.CreateDB(e.Guilds)
	if err != nil {
		log.Println("Error adding guilds: ", err)
	}
}

func messageCreated(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}
	isGuild := m.GuildID != ""

	var content string
	if strings.HasPrefix(m.Content, botMention) {
		content = strings.TrimPrefix(m.Content, botMention)
	} else if strings.HasPrefix(m.Content, database.GuildCache[m.GuildID].Prefix) {
		content = strings.TrimPrefix(m.Content, database.GuildCache[m.GuildID].Prefix)
	} else if !isGuild && strings.HasPrefix(m.Content, globalPrefix) {
		content = strings.TrimPrefix(m.Content, globalPrefix)
	} else {
		//no prefix functionality
		var err error
		if isGuild && database.GuildCache[m.GuildID].Pixiv {
			matches := utils.PixivRegex.FindAllStringSubmatch(m.Content, len(m.Content)+1)
			if matches != nil {
				ids := make([]string, 0)
				for _, match := range matches {
					ids = append(ids, match[1])
				}
				err = utils.PostPixiv(s, m, ids)
			}
		}

		if err != nil {
			log.Println(err)
			s.ChannelMessageSend(m.ChannelID, "Oops, something went wrong. Error message:\n``"+err.Error()+"``")
		}
		return
	}

	fields := strings.Fields(content)
	if len(fields) == 0 {
		return
	}

	if command, ok := commands.Commands[fields[0]]; ok {
		if !isGuild && command.GuildOnly {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%v command can't be executed in DMs or group chats", command.Name))
			return
		}
		go func() {
			in := ""
			if isGuild {
				g, _ := s.Guild(m.GuildID)
				in = g.Name
			} else {
				in = "DMs"
			}

			log.Println(fmt.Sprintf("Executing %v, requested by %v in %v", command.Name, m.Author.String(), in))
			err := command.Exec(s, m, fields[1:])
			if err != nil {
				log.Println(err)
				s.ChannelMessageSend(m.ChannelID, "Oops, something went wrong. Error message:\n``"+err.Error()+"``")
			}
		}()
	}
}

func reactCreated(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if author, ok := utils.PostCache[r.MessageID]; ok && author == r.UserID && r.Emoji.APIName() == "❌" {
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
		log.Println("Joined ", g.Name)
	}
}

func guildDeleted(s *discordgo.Session, g *discordgo.GuildDelete) {
	err := database.RemoveGuild(g.ID)
	if err != nil {
		log.Println(err)
	}

	delete(database.GuildCache, g.ID)
	log.Println("Kicked or banned from ", g.Name)
}
