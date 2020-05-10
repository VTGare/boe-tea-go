package utils

import (
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/services"
	"github.com/bwmarrin/discordgo"
)

//ActionFunc is a function type alias for prompt actions
type ActionFunc = func() bool

//PromptOptions is a struct that defines prompt's behaviour.
type PromptOptions struct {
	Actions map[string]ActionFunc
	Message string
	Timeout time.Duration
}

type PixivOptions struct {
	ProcPrompt bool
	Indexes    map[int]bool
	Exclude    bool
}

type Range struct {
	Low  int
	High int
}

var (
	EmojiRegex            = regexp.MustCompile(`(\x{00a9}|\x{00ae}|[\x{2000}-\x{3300}]|\x{d83c}[\x{d000}-\x{dfff}]|\x{d83d}[\x{d000}-\x{dfff}]|\x{d83e}[\x{d000}-\x{dfff}])`)
	NumRegex              = regexp.MustCompile(`([0-9]+)`)
	EmbedColor            = 0x439ef1
	AuthorID              = "244208152776540160"
	PixivRegex            = regexp.MustCompile(`http(?:s)?:\/\/(?:www\.)?pixiv\.net\/(?:en\/)?(?:artworks\/|member_illust\.php\?illust_id=)([0-9]+)`)
	ErrNotEnoughArguments = errors.New("not enough arguments")
	ErrParsingArgument    = errors.New("error parsing arguments, please make sure all arguments are integers")
)

func NewRange(s string) (*Range, error) {
	hyphen := strings.IndexByte(s, '-')
	if hyphen == -1 {
		return nil, errors.New("not a range")
	}
	lowStr := s[:hyphen]
	highStr := s[hyphen+1:]

	low, err := strconv.Atoi(lowStr)
	if err != nil {
		return nil, ErrParsingArgument
	}

	high, err := strconv.Atoi(highStr)
	if err != nil {
		return nil, ErrParsingArgument
	}

	if low > high {
		return nil, errors.New("low is higher than high")
	}

	return &Range{
		Low:  low,
		High: high,
	}, nil
}

func EmbedTimestamp() string {
	return time.Now().Format(time.RFC3339)
}

//FindAuthor is a SauceNAO helper function that finds original source author string.
func FindAuthor(sauce services.Sauce) string {
	if sauce.Data.MemberName != "" {
		return sauce.Data.MemberName
	} else if sauce.Data.Author != "" {
		return sauce.Data.Author
	} else if creator, ok := sauce.Data.Creator.(string); ok {
		return creator
	}

	return "-"
}

//CreatePrompt sends a prompt message to a discord channel
func CreatePrompt(s *discordgo.Session, m *discordgo.MessageCreate, opts *PromptOptions) ActionFunc {
	prompt, _ := s.ChannelMessageSend(m.ChannelID, opts.Message)
	for emoji := range opts.Actions {
		s.MessageReactionAdd(m.ChannelID, prompt.ID, emoji)
	}

	var reaction *discordgo.MessageReaction
	for {
		select {
		case k := <-nextMessageReactionAdd(s):
			reaction = k.MessageReaction
		case <-time.After(opts.Timeout):
			s.ChannelMessageDelete(prompt.ChannelID, prompt.ID)
			return nil
		}

		if _, ok := opts.Actions[reaction.Emoji.APIName()]; !ok {
			continue
		}

		if reaction.MessageID != prompt.ID || s.State.User.ID == reaction.UserID || reaction.UserID != m.Author.ID {
			continue
		}

		s.ChannelMessageDelete(prompt.ChannelID, prompt.ID)
		return opts.Actions[reaction.Emoji.APIName()]
	}
}

func nextMessageReactionAdd(s *discordgo.Session) chan *discordgo.MessageReactionAdd {
	out := make(chan *discordgo.MessageReactionAdd)
	s.AddHandlerOnce(func(_ *discordgo.Session, e *discordgo.MessageReactionAdd) {
		out <- e
	})
	return out
}

func FormatBool(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}

func CreateDB(eventGuilds []*discordgo.Guild) error {
	allGuilds := database.AllGuilds()
	for _, guild := range *allGuilds {
		database.GuildCache[guild.GuildID] = guild
	}

	newGuilds := make([]interface{}, 0)
	for _, guild := range eventGuilds {
		if _, ok := database.GuildCache[guild.ID]; !ok {
			log.Println(guild.ID, "not found in database. Adding...")
			g := database.DefaultGuildSettings(guild.ID)
			newGuilds = append(newGuilds, g)
			database.GuildCache[g.GuildID] = *g
		}
	}

	if len(newGuilds) > 0 {
		err := database.InsertManyGuilds(newGuilds)
		if err != nil {
			return err
		} else {
			log.Println("Successfully inserted all current guilds.")
		}
	}

	log.Println(fmt.Sprintf("Connected to %v guilds", len(eventGuilds)))
	return nil
}

func GetEmoji(s *discordgo.Session, guildID, e string) (string, error) {
	if EmojiRegex.MatchString(e) || e == "👌" {
		return e, nil
	}

	emojiID := NumRegex.FindString(e)
	emoji, err := s.State.Emoji(guildID, emojiID)
	if err != nil {
		return "", err
	}
	return emoji.APIName(), nil
}
