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
	embedWarning = []string{"Boe Tea has a support server now! Use bt!support command to get an invite link.", "POMF POMF KIMOCHI", "https://www.youtube.com/watch?v=899kstdMUoQ", "Do you believe in gravity?", "Shit waifu, ngl.", "Watch Monogatari.", "Is this thing on?", "Haruhi is a goddess", "My creator's waifu is 2B", "If you're reading this you're epic.", "If you react ❌ to a pixiv embed it'll be removed", "bt!nhentai 271920, enjoy", "This embed was sponsored by Raid Shadow Legends", "There are several hidden meme commands, try to find them", "Love, from Shamiko-chan", "bt!twitter is useful for mobile users", "Ramiel best girl", "PM the creator of this bot lolis.", "If you wrap a link in <> Discord won't embed it", "Who's Rem", "Every 60 seconds one minute passes in Africa", "People die when they're killed", "You thought it was a useful message, but it was me DIO!", "Enable strict mode to remove filthy reposts."}
)

type ArtPost struct {
	TwitterMatches map[string]bool
	PixivMatches   map[string]bool
	Reposts        []*database.ImagePost
	HasUgoira      bool
	event          discordgo.MessageCreate
	posts          []*ugoira.PixivPost
}

type SendPixivOptions struct {
	SkipPrompt bool
	Exclude    map[int]bool
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
func NewPost(m discordgo.MessageCreate, content ...string) *ArtPost {
	var (
		twitter = make(map[string]bool)
		IDs     = make(map[string]bool)
	)

	if len(content) != 0 {
		m.Content = content[0]
	}

	for _, str := range tsuita.TwitterRegex.FindAllString(m.Content, len(m.Content)+1) {
		twitter[str] = true
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
