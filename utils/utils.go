package utils

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

//Range is a range struct. Low is beginning value and High is end value. High can't be higher than Low.
type Range struct {
	Low  int
	High int
}

//PromptOptions is a struct that defines prompt's behaviour.
type PromptOptions struct {
	Actions map[string]bool
	Message string
	Timeout time.Duration
}

var (
	//DefaultEmbedImage is an image for embeds
	DefaultEmbedImage = "https://i.imgur.com/OZ1Al5h.png"
	//PixivRegex is a regular experession that detects various Pixiv links
	PixivRegex = regexp.MustCompile(`(?i)http(?:s)?:\/\/(?:www\.)?pixiv\.net\/(?:en\/)?(?:artworks\/|member_illust\.php\?)(?:mode=medium\&)?(?:illust_id=)?([0-9]+)`)
	//EmojiRegex matches some Unicode emojis, it's not perfect but better than nothing
	EmojiRegex = regexp.MustCompile(`(\x{00a9}|\x{00ae}|[\x{2000}-\x{3300}]|\x{d83c}[\x{d000}-\x{dfff}]|\x{d83d}[\x{d000}-\x{dfff}]|\x{d83e}[\x{d000}-\x{dfff}])`)
	//NumRegex is a terrible number regex. Gonna replace it with better code.
	NumRegex = regexp.MustCompile(`([0-9]+)`)
	//EmbedColor is a default border colour for Discord embeds
	EmbedColor = 0x439ef1
	//AuthorID is author's Discord ID, gonna replace it with an env variable.
	AuthorID = "244208152776540160"
	//ErrNotEnoughArguments is a default error when not enough arguments were given
	ErrNotEnoughArguments = errors.New("not enough arguments")
	//ErrParsingArgument is a default error when provided arguments couldn't be parsed
	ErrParsingArgument = errors.New("error parsing arguments, please make sure all arguments are integers")
	//ErrNoPermission is a default error when user doesn't have enough permissions to execute a command
	ErrNoPermission = errors.New("you don't have permissions to execute this command")
)

func Max(x, y int) int {
	if x < y {
		return y
	}
	return x
}

func Min(x, y int) int {
	if x > y {
		return y
	}
	return x
}

//MemberHasPermission checks if guild member has a permission to do something on a server.
func MemberHasPermission(s *discordgo.Session, guildID string, userID string, permission int) (bool, error) {
	member, err := s.State.Member(guildID, userID)
	if err != nil {
		if member, err = s.GuildMember(guildID, userID); err != nil {
			return false, err
		}
	}

	// Iterate through the role IDs stored in member.Roles
	// to check permissions
	for _, roleID := range member.Roles {
		role, err := s.State.Role(guildID, roleID)
		if err != nil {
			return false, err
		}
		if role.Permissions&permission != 0 {
			return true, nil
		}
	}

	return false, nil
}

//NewRange creates a new Range struct from a string. Correct format for a string is first integer-last integer (higher than first)
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

//EmbedTimestamp returns currect time formatted to RFC3339 for Discord Embeds
func EmbedTimestamp() string {
	return time.Now().Format(time.RFC3339)
}

//CreatePrompt sends a prompt message to a discord channel
func CreatePrompt(s *discordgo.Session, m *discordgo.MessageCreate, opts *PromptOptions) bool {
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
			return false
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

//CreatePromptWithMessage sends a prompt message to a discord channel
func CreatePromptWithMessage(s *discordgo.Session, m *discordgo.MessageCreate, message *discordgo.MessageSend) bool {
	var (
		prompt, _ = s.ChannelMessageSendComplex(m.ChannelID, message)
		timeout   = 45 * time.Second
		actions   = map[string]bool{"👌": true, "🙅‍♂️": false}
	)

	for emoji := range actions {
		s.MessageReactionAdd(m.ChannelID, prompt.ID, emoji)
	}

	var reaction *discordgo.MessageReaction
	for {
		select {
		case k := <-nextMessageReactionAdd(s):
			reaction = k.MessageReaction
		case <-time.After(timeout):
			s.ChannelMessageDelete(prompt.ChannelID, prompt.ID)
			return false
		}

		if _, ok := actions[reaction.Emoji.APIName()]; !ok {
			continue
		}

		if reaction.MessageID != prompt.ID || s.State.User.ID == reaction.UserID || reaction.UserID != m.Author.ID {
			continue
		}

		s.ChannelMessageDelete(prompt.ChannelID, prompt.ID)
		return actions[reaction.Emoji.APIName()]
	}
}

func nextMessageReactionAdd(s *discordgo.Session) chan *discordgo.MessageReactionAdd {
	out := make(chan *discordgo.MessageReactionAdd)
	s.AddHandlerOnce(func(_ *discordgo.Session, e *discordgo.MessageReactionAdd) {
		out <- e
	})
	return out
}

//FormatBool returns human-readable representation of boolean
func FormatBool(b bool) string {
	if b {
		return "enabled"
	}
	return "disabled"
}

//CreateDB caches all new Guilds bot's connected to in a database.
func CreateDB(eventGuilds []*discordgo.Guild) error {
	allGuilds := database.DB.AllGuilds()
	for _, guild := range allGuilds {
		database.GuildCache[guild.GuildID] = guild
	}

	newGuilds := make([]interface{}, 0)
	for _, guild := range eventGuilds {
		if _, ok := database.GuildCache[guild.ID]; !ok {
			log.Infoln(guild.ID, "not found in database. Adding...")
			g := database.DefaultGuildSettings(guild.ID)
			newGuilds = append(newGuilds, g)
			database.GuildCache[g.GuildID] = g
		}
	}

	if len(newGuilds) > 0 {
		err := database.DB.InsertManyGuilds(newGuilds)
		if err != nil {
			return err
		}
		log.Infoln("Successfully inserted all current guilds.")
	}

	log.Infoln(fmt.Sprintf("Connected to %v guilds", len(eventGuilds)))
	return nil
}

//GetEmoji returns a guild emoji API name from Discord state
func GetEmoji(s *discordgo.Session, guildID, e string) (string, error) {
	if EmojiRegex.MatchString(e) || e == "👌" {
		return e, nil
	}

	emojis, err := s.GuildEmojis(guildID)
	if err != nil {
		return "", err
	}

	for _, emoji := range emojis {
		if str := fmt.Sprintf("<:%v>", strings.ToLower(emoji.APIName())); str == e {
			return str, nil
		}
	}

	return "", errors.New("emoji not found")
}
