package bot

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/VTGare/boe-tea-go/internal/commands"
	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/repost"
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
	dg.AddHandler(bot.messageDeleted)
	dg.AddHandler(bot.guildCreated)
	dg.AddHandler(bot.guildDeleted)
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
			return err
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

	if repost.MsgCache.Count() > 0 {
		key := r.ChannelID + r.MessageID
		cache, ok := repost.MsgCache.Get(key)
		if ok {
			cache := cache.(*repost.CachedMessage)
			if cache.OriginalMessage.Author.ID != r.UserID {
				return
			}

			switch r.Emoji.APIName() {
			case "🔄":
				_, err := s.ChannelMessageEditEmbed(cache.SentMessage.ChannelID, cache.SentMessage.ID, cache.OriginalEmbed.Embed)
				if err != nil {
					log.Warnf("ChannelMessageDelete(): %v", err)
				}

				s.MessageReactionRemove(cache.SentMessage.ChannelID, cache.SentMessage.ID, "🔄", r.UserID)
			case "❌":
				err := s.ChannelMessageDelete(cache.SentMessage.ChannelID, cache.SentMessage.ID)
				if err != nil {
					log.Warnf("ChannelMessageDelete(): %v", err)
				}
			}
		}
	}
}

func (b *Bot) messageDeleted(s *discordgo.Session, m *discordgo.MessageDelete) {

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
