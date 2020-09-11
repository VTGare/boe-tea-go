package bot

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/VTGare/boe-tea-go/internal/commands"
	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/repost"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	botMention string
)

type Bot struct {
	s *discordgo.Session
}

func (b *Bot) Run() error {
	if err := b.s.Open(); err != nil {
		return err
	}

	defer b.s.Close()
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, syscall.SIGSEGV, syscall.SIGHUP)
	<-sc

	return nil
}

func NewBot(token string) (*Bot, error) {
	dg, err := discordgo.New("Bot " + token)
	if err != nil {
		return nil, err
	}

	bot := &Bot{dg}
	dg.AddHandler(bot.messageCreated)
	dg.AddHandler(bot.onReady)
	dg.AddHandler(bot.reactCreated)
	dg.AddHandler(bot.messageDeleted)
	dg.AddHandler(bot.guildCreated)
	dg.AddHandler(bot.guildDeleted)
	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAllWithoutPrivileged)
	return bot, nil
}

func (b *Bot) onReady(s *discordgo.Session, e *discordgo.Ready) {
	botMention = "<@!" + e.User.ID + ">"
	log.Infoln(e.User.String(), "is ready.")
	log.Infof("Connected to %v guilds!", len(e.Guilds))
}

func handleError(s *discordgo.Session, m *discordgo.MessageCreate, err error) {
	if err != nil {
		log.Errorf("An error occured: %v", err)
		embed := &discordgo.MessageEmbed{
			Title: "Oops, something went wrong!",
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: utils.DefaultEmbedImage,
			},
			Description: fmt.Sprintf("***Error message:***\n%v\n\nPlease contact bot's author using bt!feedback command or directly at VTGare#3370 if you can't understand the error.", err),
			Color:       utils.EmbedColor,
			Timestamp:   utils.EmbedTimestamp(),
		}
		s.ChannelMessageSendEmbed(m.ChannelID, embed)
	}
}

func (b *Bot) prefixless(s *discordgo.Session, m *discordgo.MessageCreate, crosspost bool) error {
	guild := database.GuildCache[m.GuildID]
	if !guild.Crosspost && crosspost {
		return nil
	}

	art := repost.NewPost(*m, crosspost)

	if guild.Repost != "disabled" {
		art.FindReposts()
		if len(art.Reposts) > 0 {
			if guild.Repost == "strict" {
				art.RemoveReposts()
				if crosspost {
					log.Infoln("found a repost while crossposting")
				}

				if !crosspost {
					s.ChannelMessageSendEmbed(m.ChannelID, art.RepostEmbed())
					perm, err := utils.MemberHasPermission(s, m.GuildID, s.State.User.ID, 8|8192)
					if err != nil {
						return err
					}

					if !perm {
						s.ChannelMessageSend(m.ChannelID, "Please enable Manage Messages permission to remove reposts with strict mode on, otherwise strict mode is useless.")
					} else if art.Len() == 0 {
						s.ChannelMessageDelete(m.ChannelID, m.ID)
					}
				}
			} else if guild.Repost == "enabled" && !crosspost {
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

	if guild.Pixiv && len(art.PixivMatches) > 0 {
		messages, err := art.SendPixiv(s)
		if err != nil {
			return err
		}

		embeds := make([]*discordgo.Message, 0)
		keys := make([]string, 0)
		keys = append(keys, m.Message.ID)

		for _, message := range messages {
			embed, _ := s.ChannelMessageSendComplex(m.ChannelID, message)

			if embed != nil {
				keys = append(keys, embed.ID)
				embeds = append(embeds, embed)
			}
		}

		if art.HasUgoira {
			art.Cleanup()
		}

		c := &utils.CachedMessage{m.Message, embeds}
		for _, key := range keys {
			utils.MessageCache.Set(key, c)
		}
	}

	if (guild.Twitter || crosspost) && len(art.TwitterMatches) > 0 {
		tweets, err := art.SendTwitter(s, !crosspost)
		if err != nil {
			return err
		}

		if len(tweets) > 0 {
			msg := ""
			if len(tweets) == 1 {
				msg = "Detected a tweet with more than one image, would you like to send embeds of other images for mobile users?"
			} else {
				msg = "Detected tweets with more than one image, would you like to send embeds of other images for mobile users?"
			}

			prompt := true
			if !crosspost {
				prompt = utils.CreatePrompt(s, m, &utils.PromptOptions{
					Actions: map[string]bool{
						"👌": true,
					},
					Message: msg,
					Timeout: 10 * time.Second,
				})
			}

			if prompt {
				var (
					embeds = make([]*discordgo.Message, 0)
					keys   = make([]string, 0)
				)
				keys = append(keys, m.Message.ID)

				for _, t := range tweets {
					for _, send := range t {
						embed, err := s.ChannelMessageSendComplex(m.ChannelID, send)
						if err != nil {
							log.Warnln(err)
						}

						if embed != nil {
							keys = append(keys, embed.ID)
							embeds = append(embeds, embed)
						}
					}
				}

				c := &utils.CachedMessage{m.Message, embeds}
				for _, key := range keys {
					utils.MessageCache.Set(key, c)
				}
			}
		}
	}

	return nil
}

func (b *Bot) messageCreated(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.Bot {
		return
	}

	isCommand := commands.Router.Handle(s, m)
	if !isCommand && m.GuildID != "" {
		err := b.prefixless(s, m, false)
		user := database.DB.FindUser(m.Author.ID)

		if user != nil {
			channels := user.Channels(m.ChannelID)
			for _, id := range channels {
				ch, err := s.State.Channel(id)
				if err != nil {
					log.Warnf("prefixless(): %v", err)
					return
				}

				m.ChannelID = id
				m.GuildID = ch.GuildID
				b.prefixless(s, m, true)
			}
		}

		commands.Router.ErrorHandler(err)
	}
}

func (b *Bot) reactCreated(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if utils.MessageCache.Count() > 0 && r.Emoji.APIName() == "❌" {
		if m, ok := utils.MessageCache.Get(r.MessageID); ok {
			c := m.(*utils.CachedMessage)
			if r.UserID == c.Parent.Author.ID {
				if r.MessageID == c.Parent.ID {
					s.ChannelMessageDelete(c.Parent.ChannelID, c.Parent.ID)
					utils.MessageCache.Remove(c.Parent.ID)
					for _, child := range c.Children {
						s.ChannelMessageDelete(child.ChannelID, child.ID)
						utils.MessageCache.Remove(child.ID)
					}
				} else {
					s.ChannelMessageDelete(r.ChannelID, r.MessageID)
					utils.MessageCache.Remove(r.MessageID)
				}
			}
		}
	}
}

func (b *Bot) messageDeleted(s *discordgo.Session, m *discordgo.MessageDelete) {
	if utils.MessageCache.Count() > 0 {
		if mes, ok := utils.MessageCache.Get(m.ID); ok {
			c := mes.(*utils.CachedMessage)
			if c.Parent.ID == m.ID {
				s.ChannelMessageDelete(c.Parent.ChannelID, c.Parent.ID)
				utils.MessageCache.Remove(c.Parent.ID)
				for _, child := range c.Children {
					s.ChannelMessageDelete(child.ChannelID, child.ID)
					utils.MessageCache.Remove(child.ID)
				}
			} else {
				for ind, child := range c.Children {
					if child.ID == m.ID {
						utils.MessageCache.Remove(child.ID)
						c.Children = append(c.Children[:ind], c.Children[ind+1:]...)
						break
					}
				}
			}
		}
	}
}

func (b *Bot) guildCreated(s *discordgo.Session, g *discordgo.GuildCreate) {
	if _, ok := database.GuildCache[g.ID]; !ok {
		newGuild := database.DefaultGuildSettings(g.ID)
		err := database.DB.InsertOneGuild(newGuild)
		if err != nil {
			log.Warnln(err)
		}

		database.GuildCache[g.ID] = newGuild
		log.Infoln("Joined", g.Name)
	}
}

func (b *Bot) guildDeleted(s *discordgo.Session, g *discordgo.GuildDelete) {
	/*err := database.DB.RemoveGuild(g.ID)
	if err != nil {
		log.Println(err)
	}

	delete(database.GuildCache, g.ID)*/
	log.Infoln("Kicked or banned from", g.Guild.Name, g.ID)
}
