package repost

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/ReneKroon/ttlcache"
	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/ugoira"
	"github.com/VTGare/boe-tea-go/pkg/tsuita"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

var (
	embedMessages = []*embedMessage{
		{"Interested in latest updates? Join our support server. bt!support", false},
		{"POMF POMF KIMOCHI", true},
		{"https://www.youtube.com/watch?v=899kstdMUoQ", false},
		{"Do you believe in gravity?", false},
		{"Shit waifu, ngl.", false},
		{"Watch Monogatari.", false},
		{"Is this thing on?", false},
		{"I believe in Haruhiism", false},
		{"My author's waifu is 2B, hope she doesn't kill me.", false},
		{"If you're reading this you're epic.", false},
		{"React ❌ to an embed to remove it.", false},
		{"bt!nh 271920, don't thank me.", true},
		{"This embed was sponsored by Asacoco.", false},
		{"bt!borgar 🦎🍔", false},
		{"\"Love\" © Shamiko-chan", false},
		{"Use bt!twitter to embed a Twitter post.", false},
		{"Ramiel best waifu.", false},
		{"I love Amelia.", false},
		{"I'm horni.", true},
		{"Not sure what image you posted, but you go to horny jail.", true},
		{"I swear she's legal, she said she's 600 years old.", true},
		{"Wrapping a link in <> prevents Discord from embedding it", false},
		{"Who's Rem", false},
		{"Every 60 seconds, a minute passes in Africa.", false},
		{"Every 60 minutes, an hour passes in Africa.", false},
		{"Every 24 hours, a day passes in Africa.", false},
		{"Do you remember?", false},
		{"Kiara is not a chikin", false},
		{"People die when they're killed", false},
		{"Strict repost mode removes reposts no questions asked.", false},
		{"A cat is fine too.", true},
		{"If you want to support me: https://patreon.com/vtgare", false},
		{"今昔物語集", false},
		{"⑨ ⑨ ⑨ ⑨ ⑨ ⑨ ⑨ ⑨ ⑨", false},
	}
	sfwEmbedMessages = make([]*embedMessage, 0)

	MsgCache *ttlcache.Cache
)

func init() {
	for _, m := range embedMessages {
		if !m.NSFW {
			sfwEmbedMessages = append(sfwEmbedMessages, m)
		}
	}

	MsgCache = ttlcache.NewCache()
	MsgCache.SetTTL(10 * time.Minute)
	MsgCache.SetExpirationCallback(func(key string, value interface{}) {
		cache := value.(*CachedMessage)
		m := cache.SentMessage

		cache.session.MessageReactionRemove(m.ChannelID, m.ID, "🔄", cache.session.State.User.ID)
	})
}

type ArtPost struct {
	TwitterMatches map[string]bool
	PixivMatches   map[string]bool
	HasUgoira      bool
	IsCrosspost    bool
	event          *discordgo.MessageCreate
}

type SendPixivOptions struct {
	IndexMap   map[int]bool
	Include    bool
	SkipUgoira bool
}

type embedMessage struct {
	Content string
	NSFW    bool
}

type CachedMessage struct {
	session         *discordgo.Session
	OriginalEmbed   *discordgo.MessageSend
	OriginalMessage *discordgo.MessageCreate
	SentMessage     *discordgo.Message
}

func (a *ArtPost) PixivReposts(reposts []*database.ImagePost) int {
	count := 0
	for _, rep := range reposts {
		if len(rep.Content) <= 8 {
			count++
		}
	}

	return count
}

func (a *ArtPost) PixivArray() []string {
	arr := make([]string, 0)
	for r := range a.PixivMatches {
		arr = append(arr, r)
	}

	return arr
}

func (a *ArtPost) RepostEmbed(reposts []*database.ImagePost) *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title:       "General Reposti!",
		Description: "***Reminder:*** you can look up if things you post have already been posted using Discord's search feature.\nI recommend to check reposts by post's unique identifier.",
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: utils.DefaultEmbedImage,
		},
		Timestamp: utils.EmbedTimestamp(),
		Color:     utils.EmbedColor,
	}

	for _, rep := range reposts {
		dur := rep.CreatedAt.Add(86400 * time.Second).Sub(time.Now())
		content := &discordgo.MessageEmbedField{
			Name:   "Content",
			Value:  rep.Content,
			Inline: true,
		}
		link := &discordgo.MessageEmbedField{
			Name:   "Link to post",
			Value:  fmt.Sprintf("[Press here desu~](https://discord.com/channels/%v/%v/%v)", rep.GuildID, rep.ChannelID, rep.MessageID),
			Inline: true,
		}
		expires := &discordgo.MessageEmbedField{
			Name:   "Expires",
			Value:  dur.Round(time.Second).String(),
			Inline: true,
		}
		embed.Fields = append(embed.Fields, content, link, expires)
	}

	return embed
}

func (a *ArtPost) FindReposts(guildID, channelID string) []*database.ImagePost {
	var (
		wg      sync.WaitGroup
		matches = make([]string, 0)
	)

	for str := range a.PixivMatches {
		matches = append(matches, str)
	}
	for str := range a.TwitterMatches {
		matches = append(matches, str)
	}

	resChan := make(chan *database.ImagePost, len(matches))
	wg.Add(len(matches))

	for _, match := range matches {
		go func(match string) {
			defer wg.Done()
			rep, _ := database.DB.IsRepost(channelID, match)
			if rep.Content != "" {
				resChan <- rep
			} else {
				database.DB.NewRepostDetection(a.event.Author.Username, guildID, channelID, a.event.ID, match)
			}
		}(match)
	}

	go func() {
		wg.Wait()
		close(resChan)
	}()

	reposts := make([]*database.ImagePost, 0)
	for r := range resChan {
		reposts = append(reposts, r)
	}

	return reposts
}

//Len returns a total lenght of Pixiv and Twitter matches
func (a *ArtPost) Len() int {
	return len(a.PixivMatches) + len(a.TwitterMatches)
}

//RemoveReposts removes all reposts from Pixiv and Twitter matches
func (a *ArtPost) RemoveReposts(reposts []*database.ImagePost) (pixiv, twitter map[string]bool) {
	pixiv = make(map[string]bool)
	twitter = make(map[string]bool)
	for k, v := range a.PixivMatches {
		pixiv[k] = v
	}

	for k, v := range a.TwitterMatches {
		twitter[k] = v
	}

	for _, r := range reposts {
		delete(pixiv, r.Content)
		delete(twitter, r.Content)
	}

	return
}

//Cleanup removes Ugoira files if any
func (a *ArtPost) Cleanup(posts []*ugoira.PixivPost) {
	for _, p := range posts {
		if p.Ugoira != nil && p.Ugoira.File != nil {
			logrus.Infoln("Removing Ugoira file.")
			p.Ugoira.File.Close()
			os.Remove(p.Ugoira.File.Name())
		}
	}
}

func sendMessage(s *discordgo.Session, m *discordgo.MessageCreate, send *discordgo.MessageSend) {
	msg, err := s.ChannelMessageSendComplex(m.ChannelID, send)
	if err != nil {
		logrus.Warnln(err)
	} else {
		MsgCache.Set(msg.ChannelID+msg.ID, &CachedMessage{s, send, m, msg})
		s.MessageReactionAdd(msg.ChannelID, msg.ID, "🔄")
	}
}

func (a *ArtPost) Post(s *discordgo.Session, pixivOpts ...SendPixivOptions) error {
	var (
		m       = a.event
		pixiv   = make(map[string]bool)
		twitter = make(map[string]bool)
	)

	guild := database.GuildCache[m.GuildID]
	for k, v := range a.PixivMatches {
		pixiv[k] = v
	}
	for k, v := range a.TwitterMatches {
		twitter[k] = v
	}

	if guild.Repost != "disabled" {
		reposts := a.FindReposts(m.GuildID, m.ChannelID)
		if len(reposts) > 0 {
			if guild.Repost == "strict" {
				pixiv, twitter = a.RemoveReposts(reposts)

				s.ChannelMessageSendEmbed(m.ChannelID, a.RepostEmbed(reposts))
				perm, err := utils.MemberHasPermission(s, m.GuildID, s.State.User.ID, 8|8192)
				if err != nil {
					return err
				}

				if !perm {
					s.ChannelMessageSend(m.ChannelID, "Please enable Manage Messages permission to remove reposts with strict mode on, otherwise strict mode is useless.")
				} else if len(pixiv)+len(twitter) == 0 {
					s.ChannelMessageDelete(m.ChannelID, m.ID)
				}
			} else if guild.Repost == "enabled" {
				if a.PixivReposts(reposts) > 0 && guild.Pixiv {
					prompt := utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
						Content: "Following posts are reposts, react 👌 to post them.",
						Embed:   a.RepostEmbed(reposts),
					})
					if !prompt {
						return nil
					}
				} else {
					s.ChannelMessageSendEmbed(m.ChannelID, a.RepostEmbed(reposts))
				}
			}
		}
	}

	var posts []*ugoira.PixivPost
	if guild.Pixiv && len(pixiv) > 0 {
		var (
			messages []*discordgo.MessageSend
			err      error
		)

		messages, posts, err = a.SendPixiv(s, pixiv, pixivOpts...)
		if err != nil {
			return err
		}

		for _, message := range messages {
			sendMessage(s, m, message)
		}
	}

	if guild.Twitter && len(twitter) > 0 {
		tweets, err := a.SendTwitter(s, twitter, true)
		if err != nil {
			return err
		}

		if len(tweets) > 0 {
			msg := ""

			prompt := true
			if guild.TwitterPrompt {
				if len(tweets) == 1 {
					msg = "Detected a tweet with more than one image, would you like to send embeds of other images for mobile users?"
				} else {
					msg = "Detected tweets with more than one image, would you like to send embeds of other images for mobile users?"
				}

				prompt = utils.CreatePrompt(s, m, &utils.PromptOptions{
					Actions: map[string]bool{
						"👌": true,
					},
					Message: msg,
					Timeout: 10 * time.Second,
				})
			}

			if prompt {
				for _, t := range tweets {
					for _, send := range t {
						sendMessage(s, m, send)
					}
				}
			}
		}
	}

	if a.HasUgoira {
		a.Cleanup(posts)
	}

	return nil
}

func (a *ArtPost) Crosspost(s *discordgo.Session, channels []string, pixivOpts ...SendPixivOptions) error {
	var (
		m       = a.event
		pixiv   = make(map[string]bool)
		twitter = make(map[string]bool)
	)
	a.IsCrosspost = true

	for _, id := range channels {
		ch, err := s.State.Channel(id)
		if err != nil {
			logrus.Warnf("prefixless(): %v", err)
			continue
		}

		m.ChannelID = id
		m.GuildID = ch.GuildID
		for k, v := range a.PixivMatches {
			pixiv[k] = v
		}
		for k, v := range a.TwitterMatches {
			twitter[k] = v
		}

		guild := database.GuildCache[m.GuildID]
		if guild.Repost != "disabled" {
			reposts := a.FindReposts(m.GuildID, m.ChannelID)
			pixiv, twitter = a.RemoveReposts(reposts)
		}

		if len(pixiv) > 0 {
			var (
				messages []*discordgo.MessageSend
				err      error
			)

			if len(pixivOpts) > 0 {
				pixivOpts[0].SkipUgoira = true
				messages, _, err = a.SendPixiv(s, pixiv, pixivOpts[0])
			} else {
				messages, _, err = a.SendPixiv(s, pixiv, SendPixivOptions{
					SkipUgoira: true,
				})
			}

			if err != nil {
				return err
			}

			for _, message := range messages {
				sendMessage(s, m, message)
			}
		}

		if len(twitter) > 0 {
			tweets, err := a.SendTwitter(s, twitter, false)
			if err != nil {
				return err
			}

			if len(tweets) > 0 {
				for _, t := range tweets {
					for _, send := range t {
						sendMessage(s, m, send)
					}
				}
			}
		}
	}

	return nil
}

//NewPost creates an ArtPost from discordgo message create event.
func NewPost(m *discordgo.MessageCreate, content ...string) *ArtPost {
	var (
		twitter = make(map[string]bool)
		IDs     = make(map[string]bool)
	)

	if len(content) != 0 {
		m.Content = content[0]
	}

	for _, match := range tsuita.TwitterRegex.FindAllStringSubmatch(m.Content, len(m.Content)+1) {
		twitter[match[1]] = true
	}

	pixiv := utils.PixivRegex.FindAllStringSubmatch(m.Content, len(m.Content)+1)
	if pixiv != nil {
		for _, match := range pixiv {
			IDs[match[1]] = true
		}
	}

	return &ArtPost{
		event:          m,
		TwitterMatches: twitter,
		PixivMatches:   IDs,
	}
}
