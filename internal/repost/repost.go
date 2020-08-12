package repost

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/ugoira"
	"github.com/VTGare/boe-tea-go/services"
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
			URL: "https://i.imgur.com/OZ1Al5h.png",
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
			rep, _ := utils.IsRepost(a.event.ChannelID, match)
			if rep.Content != "" {
				resChan <- rep
			} else {
				utils.NewRepostDetection(a.event.Author.Username, a.event.GuildID, a.event.ChannelID, a.event.ID, match)
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
			p.Ugoira.File.Close()
			os.Remove(p.Ugoira.File.Name())
		}
	}
}

func (a *ArtPost) fetchPixivPosts() ([]*ugoira.PixivPost, error) {
	var (
		errChan  = make(chan error)
		postChan = make(chan *ugoira.PixivPost, len(a.PixivMatches))
		wg       sync.WaitGroup
	)

	wg.Add(len(a.PixivMatches))
	for id := range a.PixivMatches {
		go func(id string) {
			defer wg.Done()
			px, err := ugoira.GetPixivPost(id)
			if err != nil {
				errChan <- err
			}
			postChan <- px
		}(id)
	}

	go func() {
		wg.Wait()
		close(errChan)
		close(postChan)
	}()

	for err := range errChan {
		return nil, err
	}

	posts := make([]*ugoira.PixivPost, 0)
	for post := range postChan {
		posts = append(posts, post)
	}

	return posts, nil
}

func countPages(posts []*ugoira.PixivPost) int {
	count := 0
	for _, p := range posts {
		count += utils.Max(len(p.LargeImages), len(p.OriginalImages))
	}
	return count
}

func isNSFW(posts []*ugoira.PixivPost) bool {
	for _, p := range posts {
		if p.NSFW {
			return true
		}
	}
	return false
}

func (a *ArtPost) SendPixiv(s *discordgo.Session, opts ...SendPixivOptions) ([]*discordgo.MessageSend, error) {
	var (
		guild      = database.GuildCache[a.event.GuildID]
		exclude    = make(map[int]bool)
		skipPrompt bool
		err        error
	)
	if len(opts) != 0 {
		if opts[0].Exclude != nil {
			exclude = opts[0].Exclude
		}
		skipPrompt = opts[0].SkipPrompt
	}

	a.posts, err = a.fetchPixivPosts()
	if err != nil {
		return nil, err
	}

	count := countPages(a.posts) - len(exclude)
	if count >= guild.LargeSet && !skipPrompt {
		prompt := utils.CreatePrompt(s, &a.event, &utils.PromptOptions{
			Actions: map[string]bool{
				"👌": true,
			},
			Message: fmt.Sprintf("Album size ***(%v)*** is larger than large set setting ***(%v)***, please confirm the operation.", count, guild.LargeSet),
			Timeout: 15 * time.Second,
		})
		if !prompt {
			return nil, nil
		}
	}

	ch, err := s.Channel(a.event.ChannelID)
	if err != nil {
		return nil, err
	}
	if isNSFW(a.posts) && !ch.NSFW {
		prompt := utils.CreatePrompt(s, &a.event, &utils.PromptOptions{
			Actions: map[string]bool{
				"👌": true,
			},
			Message: fmt.Sprintf("You're trying to send an NSFW post in a SFW channel, are you sure about that?"),
			Timeout: 15 * time.Second,
		})
		if !prompt {
			return nil, nil
		}
	}

	return createEmbeds(a, exclude, guild), nil
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

	for _, str := range services.TwitterRegex.FindAllString(m.Content, len(m.Content)+1) {
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

func joinTags(elems []string, sep string) string {
	switch len(elems) {
	case 0:
		return ""
	case 1:
		return elems[0]
	}
	n := len(sep) * (len(elems) - 1)
	for i := 0; i < len(elems); i++ {
		n += len(elems[i])
	}

	var b strings.Builder
	b.Grow(n)
	b.WriteString(fmt.Sprintf("[%v](https://www.pixiv.net/en/tags/%v/artworks)", elems[0], elems[0]))
	for _, s := range elems[1:] {
		b.WriteString(sep)
		b.WriteString(fmt.Sprintf("[%v](https://www.pixiv.net/en/tags/%v/artworks)", s, s))
	}
	return b.String()
}

func createEmbeds(a *ArtPost, excluded map[int]bool, guild *database.GuildSettings) []*discordgo.MessageSend {
	var (
		easterEgg    = rand.Intn(len(embedWarning))
		createdCount = 0
		messages     = make([]*discordgo.MessageSend, 0)
	)

	count := countPages(a.posts)
	for _, post := range a.posts {
		if createdCount == guild.Limit {
			break
		}

		for ind, thumbnail := range post.LargeImages {
			if _, ok := excluded[ind+1]; ok {
				continue
			}
			createdCount++

			var ms *discordgo.MessageSend
			if post.Type == "ugoira" {
				err := post.DownloadUgoira()
				if err != nil {
					logrus.Warnln(err)
					ms = createEmbed(post, thumbnail, post.OriginalImages[ind], ind, easterEgg)
				} else {
					a.HasUgoira = true
					ms = createUgoiraEmbed(post, easterEgg)
				}
			} else {
				ms = createEmbed(post, thumbnail, post.OriginalImages[ind], ind, easterEgg)
			}
			messages = append(messages, ms)

			if count >= guild.Limit {
				break
			}
		}
	}

	if count > guild.Limit {
		messages[0].Content = fmt.Sprintf("```Album size (%v) is larger than limit set on this server (%v), only first image of every post is reposted.```", count, guild.Limit)
	}

	return messages
}

func createEmbed(post *ugoira.PixivPost, thumbnail, original string, ind, easterEgg int) *discordgo.MessageSend {
	title := ""

	if len(post.LargeImages) == 1 {
		title = fmt.Sprintf("%v by %v", post.Title, post.Author)
	} else {
		title = fmt.Sprintf("%v by %v. Page %v/%v", post.Title, post.Author, ind+1, len(post.LargeImages))
	}

	send := &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Title:       title,
			URL:         fmt.Sprintf("https://www.pixiv.net/en/artworks/%v", post.ID),
			Color:       utils.EmbedColor,
			Timestamp:   utils.EmbedTimestamp(),
			Description: fmt.Sprintf("[Original quality](%v)", original),
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Likes",
					Value:  strconv.Itoa(post.Likes),
					Inline: true,
				},
				{
					Name:   "Tags",
					Value:  joinTags(post.Tags, " • "),
					Inline: true,
				},
			},
			Image: &discordgo.MessageEmbedImage{
				URL: thumbnail,
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: embedWarning[easterEgg],
			},
		},
	}

	return send
}

func createUgoiraEmbed(post *ugoira.PixivPost, easterEgg int) *discordgo.MessageSend {
	title := fmt.Sprintf("%v by %v", post.Title, post.Author)
	send := &discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Title:     title,
			URL:       fmt.Sprintf("https://www.pixiv.net/en/artworks/%v", post.ID),
			Color:     utils.EmbedColor,
			Timestamp: utils.EmbedTimestamp(),
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Likes",
					Value:  strconv.Itoa(post.Likes),
					Inline: true,
				},
				{
					Name:   "Tags",
					Value:  joinTags(post.Tags, " • "),
					Inline: true,
				},
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: embedWarning[easterEgg],
			},
		},
	}

	send.Files = append(send.Files, &discordgo.File{
		Name:   fmt.Sprintf("%v.mp4", post.ID),
		Reader: post.Ugoira.File,
	})
	return send
}
