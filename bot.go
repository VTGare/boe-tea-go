package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/commands"
	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/pixivhelper"
	"github.com/VTGare/boe-tea-go/services"
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
	if guild.Pixiv {
		matches := pixivhelper.Regex.FindAllStringSubmatch(m.Content, len(m.Content)+1)
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

	if tweets := services.TwitterRegex.FindAllString(m.Content, len(m.Content)+1); tweets != nil {
		repostSetting := guild.Repost
		if repostSetting != "disabled" {
			for _, tweet := range tweets {
				repost, err := utils.IsRepost(m.ChannelID, tweet)
				if err != nil {
					return err
				}

				if repost != nil {
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
					utils.NewRepostDetection(m.Author.Username, m.GuildID, m.ChannelID, m.ID, tweet)
				}
			}
		}
	}
	return nil
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

	var content = trimPrefix(m.Content, m.GuildID)
	if content == m.Content && isGuild {
		err := prefixless(s, m)
		if err != nil {
			handleError(s, m, err)
		}
		return
	}

	fields := strings.Fields(content)
	if len(fields) == 0 {
		return
	}

	for _, group := range commands.CommandGroups {
		if command, ok := group.Commands[fields[0]]; ok {
			if !isGuild && command.GuildOnly {
				s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%v command can't be executed in DMs or group chats", command.Name))
				return
			}
			go func() {
				log.Infof("Executing %v, requested by %v in %v", m.Content, m.Author.String(), where())
				err := command.Exec(s, m, fields[1:])
				handleError(s, m, err)
			}()

			break
		}
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
