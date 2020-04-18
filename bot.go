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
	fmt.Println(e.User.String(), "is ready.")

	allGuilds := database.AllGuilds()
	for _, guild := range *allGuilds {
		database.GuildCache[guild.GuildID] = guild
	}

	guilds := make([]interface{}, 0)
	for _, guild := range e.Guilds {
		log.Println("Connected to", guild.Name)

		if _, ok := database.GuildCache[guild.ID]; !ok {
			log.Println(guild.Name, "not found in database. Adding...")
			guilds = append(guilds, database.DefaultGuildSettings(guild.ID))
		}
	}
	if len(guilds) > 0 {
		err := database.InserManyGuilds(guilds)
		if err != nil {
			log.Println("Error adding documents", err)
		} else {
			log.Println("Successfully inserted all current guilds.")
		}
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
	} else {
		//no prefix functionality
		in := ""
		if isGuild {
			g, _ := s.Guild(m.GuildID)
			in = g.Name
		} else {
			in = "DMs"
		}
		log.Println(fmt.Sprintf("Reposting Pixiv images in %v, requested by %v", in, m.Author.String()))
		utils.PostPixiv(s, m, m.Content)
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
				s.ChannelMessageSend(m.ChannelID, "Oops, something went wrong. Error message:\n``"+err.Error()+"``")
			}
		}()
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
