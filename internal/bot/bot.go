package bot

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/VTGare/boe-tea-go/internal/commands"
	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/repost"
	"github.com/VTGare/boe-tea-go/internal/ugoira"
	"github.com/VTGare/boe-tea-go/pkg/tsuita"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	botMention string
	BoeTea     *Bot
)

type Bot struct {
	Session *discordgo.Session
}

func (b *Bot) Run() error {
	if err := b.Session.Open(); err != nil {
		return err
	}

	defer b.Session.Close()
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
	dg.AddHandler(bot.guildCreated)
	dg.AddHandler(bot.guildDeleted)
	dg.AddHandler(bot.reactRemoved)
	dg.Identify.Intents = discordgo.MakeIntent(discordgo.IntentsAllWithoutPrivileged)

	BoeTea = bot
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
			Description: fmt.Sprintf("***Error message:***\n%v\n\nPlease contact bot's author using bt!feedback command or directly at VTGare#3599 if you can't understand the error.", err),
			Color:       utils.EmbedColor,
			Timestamp:   utils.EmbedTimestamp(),
		}
		s.ChannelMessageSendEmbed(m.ChannelID, embed)
	}
}

func (b *Bot) prefixless(s *discordgo.Session, m *discordgo.MessageCreate) error {
	var (
		art = repost.NewPost(m)
	)

	err := art.Post(s)
	if err != nil {
		log.Warnln("art.Post():", err)
	}

	if user := database.DB.FindUser(m.Author.ID); user != nil {
		channels := user.Channels(m.ChannelID)
		err := art.Crosspost(s, channels)
		if err != nil {
			log.Warnln("art.Crosspost():", err)
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
		err := b.prefixless(s, m)
		commands.Router.ErrorHandler(err)
	}
}

func (b *Bot) reactCreated(s *discordgo.Session, r *discordgo.MessageReactionAdd) {
	if r.UserID == s.State.User.ID {
		return
	}

	addFavourite := func(nsfw bool) {
		user := database.DB.FindUser(r.UserID)
		if user == nil {
			user = database.NewUserSettings(r.UserID)
			database.DB.InsertOneUser(user)
		}

		if msg, err := s.ChannelMessage(r.ChannelID, r.MessageID); err != nil {
			log.Warnf("reactCreated() -> s.ChannelMessage(): %v", err)
		} else {
			art := repost.NewPost(&discordgo.MessageCreate{msg})
			if len(msg.Embeds) == 0 && art.Len() == 0 {
				return
			}

			var favourite *database.Favourite
			switch {
			case len(art.PixivMatches) > 0:
				pixivID := ""
				for k := range art.PixivMatches {
					pixivID = k
					break
				}

				pixiv, err := ugoira.GetPixivPost(pixivID)
				if err != nil {
					log.Warnf("addFavorite -> GetPixivPost: %v", err)
					s.ChannelMessageSendComplex(r.ChannelID, commands.Router.ErrorHandler(fmt.Errorf("Error while adding a favourite: %v", err)))
					return
				}

				favourite = &database.Favourite{
					Title:     pixiv.Title,
					Author:    pixiv.Author,
					Thumbnail: pixiv.Images.Preview[0].Kotori,
					URL:       fmt.Sprintf("https://pixiv.net/en/artworks/%v", pixiv.ID),
					NSFW:      nsfw,
					CreatedAt: time.Now(),
				}
			case len(art.TwitterMatches) > 0:
				twitterURL := ""
				for k := range art.TwitterMatches {
					twitterURL = "https://twitter.com/i/status/" + k
					break
				}

				tweet, err := tsuita.GetTweet(twitterURL)
				if err != nil {
					log.Warnf("addFavorite -> GetPixivPost: %v", err)
					s.ChannelMessageSendComplex(r.ChannelID, commands.Router.ErrorHandler(fmt.Errorf("Error while adding a favourite: %v", err)))
					return
				}

				if len(tweet.Gallery) > 0 {
					favourite = &database.Favourite{
						Author:    tweet.Username,
						Thumbnail: tweet.Gallery[0].URL,
						URL:       tweet.URL,
						NSFW:      nsfw,
						CreatedAt: time.Now(),
					}
				}
			case len(msg.Embeds) != 0:
				embed := msg.Embeds[0]
				switch {
				case strings.Contains(embed.URL, "twitter") && strings.Contains(embed.Footer.Text, "Twitter"):
					favourite = &database.Favourite{
						Author:    embed.Title[strings.Index(embed.Title, "@")+1 : strings.LastIndex(embed.Title, ")")],
						Thumbnail: embed.Image.URL,
						URL:       embed.URL,
						NSFW:      nsfw,
						CreatedAt: time.Now(),
					}
				case strings.Contains(embed.URL, "pixiv") && strings.Contains(embed.Title, "by"):
					last := strings.LastIndex(embed.Title, ".")
					if last == -1 {
						last = len(embed.Title)
					}

					favourite = &database.Favourite{
						Title:     embed.Title[:strings.LastIndex(embed.Title, " by ")],
						Author:    embed.Title[strings.LastIndex(embed.Title, " by ")+4 : last],
						Thumbnail: embed.Image.URL,
						URL:       embed.URL,
						NSFW:      nsfw,
						CreatedAt: time.Now(),
					}
				}
			}

			if favourite != nil {
				database.DB.CreateFavourite(r.UserID, favourite)
			}
		}
	}

	switch r.Emoji.APIName() {
	case "❌":
		if repost.MsgCache.Count() > 0 {
			key := r.ChannelID + r.MessageID
			cache, ok := repost.MsgCache.Get(key)
			if ok {
				cache := cache.(*repost.CachedMessage)
				if cache.OriginalMessage.Author.ID != r.UserID {
					return
				}
				err := s.ChannelMessageDelete(cache.SentMessage.ChannelID, cache.SentMessage.ID)
				if err != nil {
					log.Warnf("ChannelMessageDelete(): %v", err)
				}
			}
		}
	case "💖":
		addFavourite(false)
	case "🤤":
		addFavourite(true)
	}
}

func (b *Bot) reactRemoved(s *discordgo.Session, r *discordgo.MessageReactionRemove) {
	if r.UserID == s.State.User.ID {
		return
	}

	if r.Emoji.APIName() != "💖" && r.Emoji.APIName() != "🤤" {
		return
	}

	user := database.DB.FindUser(r.UserID)
	if user != nil {
		if msg, err := s.ChannelMessage(r.ChannelID, r.MessageID); err != nil {
			log.Warnf("reactCreated() -> s.ChannelMessage(): %v", err)
		} else {
			art := repost.NewPost(&discordgo.MessageCreate{msg})
			if len(msg.Embeds) == 0 && art.Len() == 0 {
				return
			}

			switch {
			case len(art.PixivMatches) > 0:
				pixivID := ""
				for k := range art.PixivMatches {
					pixivID = k
					break
				}

				if f, _ := database.DB.DeleteFavouriteURL(user.ID, "https://pixiv.net/en/artworks/"+pixivID); !f {
					database.DB.DeleteFavouriteURL(user.ID, "https://pixiv.net/artworks/"+pixivID)
				}
			case len(art.TwitterMatches) > 0:
				twitterURL := ""
				for k := range art.TwitterMatches {
					twitterURL = "https://twitter.com/i/status/" + k
					break
				}

				tweet, err := tsuita.GetTweet(twitterURL)
				if err != nil {
					log.Warnf("reactRemoved -> GetTweet: %v", err)
					s.ChannelMessageSendComplex(r.ChannelID, commands.Router.ErrorHandler(fmt.Errorf("Error while adding a favourite: %v", err)))
					return
				}

				database.DB.DeleteFavouriteURL(user.ID, tweet.URL)
			case len(msg.Embeds) != 0:
				embed := msg.Embeds[0]
				switch {
				case strings.Contains(embed.URL, "twitter") && strings.Contains(embed.Footer.Text, "Twitter"):
					database.DB.DeleteFavouriteURL(user.ID, embed.URL)
				case strings.Contains(embed.URL, "pixiv") && strings.Contains(embed.Title, "by"):
					database.DB.DeleteFavouriteURL(user.ID, embed.URL)
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
	if !g.Unavailable {
		log.Infoln("Kicked/banned from a guild. ID: ", g.ID)
	} else {
		log.Infoln("Guild outage. ID: ", g.ID)
	}
}
