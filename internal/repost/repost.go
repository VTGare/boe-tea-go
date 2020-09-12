package repost

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/ugoira"
	"github.com/VTGare/boe-tea-go/pkg/tsuita"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

var (
	embedMessages = []*embedMessage{
		{"Come join Boe Tea's support server. Use bt!support command to get an invite link.", false},
		{"Interested in latest updates? Join our support server. bt!support", false},
		{"POMF POMF KIMOCHI", true},
		{"https://www.youtube.com/watch?v=899kstdMUoQ", false},
		{"Do you believe in gravity?", false},
		{"Shit waifu, ngl", false},
		{"Watch Monogatari.", false},
		{"Is this thing on?", false},
		{"I believe in Haruhiism", false},
		{"My author's waifu is 2B, hope she doesn't kill me.", false},
		{"If you're reading this you're epic.", false},
		{"React ❌ to an embed to remove it.", false},
		{"bt!nh 271920, don't thank me.", true},
		{"This embed was sponsored by Said Rhadow Legends", false},
		{"bt!borgar 🦎🍔", false},
		{"Love. From Shamiko-chan", false},
		{"Use bt!twitter to embed a Twitter post", false},
		{"Ramiel - best waifu.", false},
		{"DM creator of this bot lolis", true},
		{"Wrapping a link in <> prevents Discord from embedding it", false},
		{"Who's Rem", false},
		{"Every 60 seconds, a minute passes in Africa.", false},
		{"People die when they're killed", false},
		{"You thought it was a funny message, but it was me JOJO REFERENCE", false},
		{"Strict repost mode removes reposts no questions asked.", false},
		{"A cat is fine too.", true},
		{"If you want to buy me a coffee: https://ko-fi.com/vtgare", false},
		{"If you want to buy me a coffee: https://ko-fi.com/vtgare", false},
	}
	sfwEmbedMessages = make([]*embedMessage, 0)
)

func init() {
	for _, m := range embedMessages {
		if !m.NSFW {
			sfwEmbedMessages = append(sfwEmbedMessages, m)
		}
	}
}

type ArtPost struct {
	TwitterMatches map[string]bool
	PixivMatches   map[string]bool
	Reposts        []*database.ImagePost
	HasUgoira      bool
	Crosspost      bool
	event          discordgo.MessageCreate
	posts          []*ugoira.PixivPost
}

type SendPixivOptions struct {
	SkipPrompt bool
	Exclude    map[int]bool
}

type embedMessage struct {
	Content string
	NSFW    bool
}

func (a *ArtPost) PixivReposts() int {
	count := 0
	for _, rep := range a.Reposts {
		if !strings.Contains(rep.Content, "twitter") {
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

func (a *ArtPost) RepostEmbed() *discordgo.MessageEmbed {
	embed := &discordgo.MessageEmbed{
		Title: "General Reposti!",
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: utils.DefaultEmbedImage,
		},
		Timestamp: utils.EmbedTimestamp(),
		Color:     utils.EmbedColor,
	}

	for _, rep := range a.Reposts {
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

func (a *ArtPost) FindReposts() {
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
			rep, _ := database.DB.IsRepost(a.event.ChannelID, match)
			if rep.Content != "" {
				resChan <- rep
			} else {
				database.DB.NewRepostDetection(a.event.Author.Username, a.event.GuildID, a.event.ChannelID, a.event.ID, match)
			}
		}(match)
	}

	go func() {
		wg.Wait()
		close(resChan)
	}()

	for r := range resChan {
		a.Reposts = append(a.Reposts, r)
	}
}

//Len returns a total lenght of Pixiv and Twitter matches
func (a *ArtPost) Len() int {
	return len(a.PixivMatches) + len(a.TwitterMatches)
}

//RemoveReposts removes all reposts from Pixiv and Twitter matches
func (a *ArtPost) RemoveReposts() {
	for _, r := range a.Reposts {
		delete(a.PixivMatches, r.Content)
		delete(a.TwitterMatches, r.Content)
	}
}

//Cleanup removes Ugoira files if any
func (a *ArtPost) Cleanup() {
	for _, p := range a.posts {
		if p.Ugoira != nil && p.Ugoira.File != nil {
			logrus.Infoln("Removing Ugoira file.")
			p.Ugoira.File.Close()
			os.Remove(p.Ugoira.File.Name())
		}
	}
}

//NewPost creates an ArtPost from discordgo message create event.
func NewPost(m discordgo.MessageCreate, crosspost bool, content ...string) *ArtPost {
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
		Crosspost:      crosspost,
		TwitterMatches: twitter,
		PixivMatches:   IDs,
	}
}
