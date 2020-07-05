package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/commands"
	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/pixiv"
	"github.com/VTGare/boe-tea-go/services"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	botMention   string
	globalPrefix = "bt!"
)

func onReady(s *discordgo.Session, e *discordgo.Ready) {
	botMention = "<@!" + e.User.ID + ">"
	log.Infoln(e.User.String(), "is ready.")

	err := utils.CreateDB(e.Guilds)
	if err != nil {
		log.Warnln("Error adding guilds: ", err)
	}
}

func messageCreated(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}
	isGuild := m.GuildID != ""
	m.Content = strings.ToLower(m.Content)

	where := func() string {
		if isGuild {
			g, _ := s.Guild(m.GuildID)
			return g.Name
		}
		return "DMs"
	}

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
			matches := pixiv.Regex.FindAllStringSubmatch(m.Content, len(m.Content)+1)
			if matches != nil {
				ids := make([]string, 0)
				for _, match := range matches {
					ids = append(ids, match[1])
				}

				log.Infof("Found a pixiv link on %v (%v), channel %v", where(), m.GuildID, m.ChannelID)
				err = pixiv.PostPixiv(s, m, ids)
			}
		}

		if twitter := services.TwitterRegex.FindAllString(m.Content, len(m.Content)+1); isGuild && twitter != nil {
			repostSetting := database.GuildCache[m.GuildID].Repost
			if repostSetting != "disabled" {
				for _, tweet := range twitter {
					if utils.IsRepost(m.ChannelID, tweet) {
						f, _ := utils.MemberHasPermission(s, m.GuildID, s.State.User.ID, discordgo.PermissionManageMessages|discordgo.PermissionAdministrator)

						if f && repostSetting == "strict" {
							err := s.ChannelMessageDelete(m.ChannelID, m.ID)
							if err != nil {
								log.Warn(err)
							}
						}
						tweet, _ := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Repost detected. This tweet has been posted before within the last 24 hours."))
						go func() {
							time.Sleep(15 * time.Second)
							s.ChannelMessageDelete(tweet.ChannelID, tweet.ID)
						}()
					} else {
						utils.NewRepostChecker(m.ChannelID, tweet)
					}
				}
			}
		}

		if err != nil {
			log.Warnln(err)
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
			log.Infof("Executing %v, requested by %v in %v", m.Content, m.Author.String(), where())
			err := command.Exec(s, m, fields[1:])
			if err != nil {
				log.Println(err)
				s.ChannelMessageSend(m.ChannelID, "Oops, something went wrong. Error message:\n```"+err.Error()+"```")
			}
		}()
	}
}

func reactCreated(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if author, ok := pixiv.EmbedCache[r.MessageID]; ok && author == r.UserID && r.Emoji.APIName() == "❌" {
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
