package repost

import (
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/ugoira"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	"github.com/sirupsen/logrus"
)

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
		guild   = database.GuildCache[a.event.GuildID]
		exclude = make(map[int]bool)

		err error
	)
	if len(opts) != 0 {
		if opts[0].Exclude != nil {
			exclude = opts[0].Exclude
		}
	}

	a.posts, err = a.fetchPixivPosts()
	if err != nil {
		return nil, err
	}

	for excl := range exclude {
		if excl < 0 || excl > countPages(a.posts) {
			delete(exclude, excl)
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

	return createPixivEmbeds(a, exclude, guild), nil
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

func createPixivEmbeds(a *ArtPost, excluded map[int]bool, guild *database.GuildSettings) []*discordgo.MessageSend {
	var (
		easterEgg    = rand.Intn(len(embedWarning))
		createdCount = 0
		messages     = make([]*discordgo.MessageSend, 0)
	)

	count := countPages(a.posts) - len(excluded)
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
					ms = createPixivEmbed(post, thumbnail, post.OriginalImages[ind], ind, easterEgg)
				} else {
					a.HasUgoira = true
					ms = createUgoiraEmbed(post, easterEgg)
				}
			} else {
				ms = createPixivEmbed(post, thumbnail, post.OriginalImages[ind], ind, easterEgg)
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

	if a.Crosspost {
		for _, m := range messages {
			m.Embed.Author = &discordgo.MessageEmbedAuthor{Name: fmt.Sprintf("Crosspost requested by %v", a.event.Author.String()), IconURL: a.event.Author.AvatarURL("")}
		}
	}

	return messages
}

func createPixivEmbed(post *ugoira.PixivPost, thumbnail, original string, ind, easterEgg int) *discordgo.MessageSend {
	title := ""

	if len(post.LargeImages) == 1 {
		title = fmt.Sprintf("%v by %v", post.Title, post.Author)
	} else {
		title = fmt.Sprintf("%v by %v. Page %v/%v", post.Title, post.Author, ind+1, len(post.LargeImages))
	}

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
					Name:   "Original quality",
					Value:  fmt.Sprintf("[Click here desu~](%v)", original),
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

	if ind == 0 {
		send.Embed.Description = fmt.Sprintf("**Tags**\n%v", joinTags(post.Tags, " • "))
	}

	if post.GoodWaifu && strings.Contains(send.Embed.Footer.Text, "Shit waifu") {
		send.Embed.Footer.Text = "Good taste, mate."
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
