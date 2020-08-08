package main

import (
	"fmt"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/VTGare/boe-tea-go/commands"
	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/repost"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	botMention      string
	defaultPrefixes = []string{"bt!", "bt.", "bt "}
	messageCache    *ttlcache.Cache
)

type cachedMessage struct {
	Parent   *discordgo.Message
	Children []*discordgo.Message
}

func init() {
	messageCache = ttlcache.NewCache()
	messageCache.SetTTL(15 * time.Minute)
}

func onReady(s *discordgo.Session, e *discordgo.Ready) {
	botMention = "<@!" + e.User.ID + ">"
	log.Infoln(e.User.String(), "is ready.")

	err := utils.CreateDB(e.Guilds)
	if err != nil {
		log.Warnln("Error adding guilds: ", err)
	}
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
	art := repost.NewPost(*m)
	guild := database.GuildCache[m.GuildID]

	if guild.Repost != "disabled" {
		art.FindReposts()
		if len(art.Reposts) > 0 {
			if guild.Repost == "strict" {
				art.RemoveReposts()
				s.ChannelMessageSendEmbed(m.ChannelID, art.RepostEmbed())
				if art.Len() == 0 {

					s.ChannelMessageDelete(m.ChannelID, m.ID)
				}
			} else if guild.Repost == "enabled" {
				if art.PixivReposts() > 0 && guild.Pixiv {
					prompt := utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
						Content: "Following posts are reposts, react 👌 to post them.",
						Embed:   art.RepostEmbed(),
					})
					if !prompt {
						return nil
					}
				} else {
					s.ChannelMessageSendEmbed(m.ChannelID, art.RepostEmbed())
				}
			}
		}
	}

	if guild.Pixiv {
		messages, err := art.SendPixiv(s)
		if err != nil {
			return err
		}

		embeds := make([]*discordgo.Message, 0)
		keys := make([]string, 0)
		keys = append(keys, m.Message.ID)

		for _, message := range messages {
			embed, _ := s.ChannelMessageSendComplex(m.ChannelID, message)

			keys = append(keys, embed.ID)
			embeds = append(embeds, embed)
		}

		if art.HasUgoira {
			art.Cleanup()
		}

		c := &cachedMessage{m.Message, embeds}
		for _, key := range keys {
			messageCache.Set(key, c)
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
	if messageCache.Count() > 0 && r.Emoji.APIName() == "❌" {
		if m, ok := messageCache.Get(r.MessageID); ok {
			c := m.(*cachedMessage)
			if r.UserID == c.Parent.Author.ID {
				if r.MessageID == c.Parent.ID {
					s.ChannelMessageDelete(c.Parent.ChannelID, c.Parent.ID)
					messageCache.Remove(c.Parent.ID)
					for _, child := range c.Children {
						s.ChannelMessageDelete(child.ChannelID, child.ID)
						messageCache.Remove(child.ID)
					}
				} else {
					s.ChannelMessageDelete(r.ChannelID, r.MessageID)
					messageCache.Remove(r.MessageID)
				}
			}
		}
	}
}

func messageDeleted(s *discordgo.Session, m *discordgo.MessageDelete) {
	if messageCache.Count() > 0 {
		if mes, ok := messageCache.Get(m.ID); ok {
			c := mes.(*cachedMessage)
			if c.Parent.ID == m.ID {
				s.ChannelMessageDelete(c.Parent.ChannelID, c.Parent.ID)
				messageCache.Remove(c.Parent.ID)
				for _, child := range c.Children {
					s.ChannelMessageDelete(child.ChannelID, child.ID)
					messageCache.Remove(child.ID)
				}
			}
		}
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
