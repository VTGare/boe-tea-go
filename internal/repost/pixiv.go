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

func (a *ArtPost) fetchPixivPosts(IDs map[string]bool) ([]*ugoira.PixivPost, error) {
	var (
		errChan  = make(chan error)
		postChan = make(chan *ugoira.PixivPost, len(a.PixivMatches))
		wg       sync.WaitGroup
	)

	wg.Add(len(IDs))
	for id := range IDs {
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
		count += p.Len()
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

func (a *ArtPost) SendPixiv(s *discordgo.Session, IDs map[string]bool, opts ...SendPixivOptions) ([]*discordgo.MessageSend, []*ugoira.PixivPost, error) {
	var (
		guild      = database.GuildCache[a.event.GuildID]
		indexMap   = make(map[int]bool)
		include    bool
		skipUgoira bool
		err        error
	)

	if len(opts) != 0 {
		if opts[0].IndexMap != nil {
			indexMap = opts[0].IndexMap
			include = opts[0].Include
		}
		skipUgoira = opts[0].SkipUgoira
	}

	posts, err := a.fetchPixivPosts(IDs)
	if err != nil {
		return nil, nil, err
	}

	if len(opts) != 0 {
		if opts[0].SkipUgoira {

		}
	}

	for excl := range indexMap {
		if excl < 0 || excl > countPages(posts) {
			delete(indexMap, excl)
		}
	}

	if isNSFW(posts) {
		if !guild.NSFW {
			s.ChannelMessageSendEmbed(a.event.ChannelID, &discordgo.MessageEmbed{
				Title:     "❎ Pixiv post has not been reposted.",
				Color:     utils.EmbedColor,
				Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
				Timestamp: utils.EmbedTimestamp(),
				Fields:    []*discordgo.MessageEmbedField{{"Reason", "An NSFW post has been detected. The server prohibits NSFW content.", false}},
			})

			return nil, nil, nil
		}
		ch, err := s.Channel(a.event.ChannelID)
		if err != nil {
			return nil, nil, err
		}

		if !ch.NSFW {
			prompt := utils.CreatePrompt(s, a.event, &utils.PromptOptions{
				Actions: map[string]bool{
					"👌": true,
				},
				Message: fmt.Sprintf("You're trying to send an NSFW post in a SFW channel, are you sure about that?"),
				Timeout: 15 * time.Second,
			})
			if !prompt {
				return nil, nil, nil
			}
		}
	}

	return createPixivEmbeds(a, posts, indexMap, include, skipUgoira, guild), posts, nil
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

func createPixivEmbeds(a *ArtPost, posts []*ugoira.PixivPost, indexMap map[int]bool, include, skipUgoira bool, guild *database.GuildSettings) []*discordgo.MessageSend {
	var (
		easterEgg    *embedMessage
		createdCount = 0
		messages     = make([]*discordgo.MessageSend, 0)
	)

	g := database.GuildCache[a.event.GuildID]
	if !g.NSFW {
		easterEgg = sfwEmbedMessages[rand.Intn(len(sfwEmbedMessages))]
	} else {
		easterEgg = embedMessages[rand.Intn(len(embedMessages))]
	}

	count := countPages(posts) - len(indexMap)
	for _, post := range posts {
		if createdCount == guild.Limit {
			break
		}

		for ind := range post.Images.Original {
			if _, ok := indexMap[ind+1]; ok != include {
				continue
			}
			createdCount++

			var ms *discordgo.MessageSend
			if post.Type == "ugoira" && !skipUgoira {
				err := post.DownloadUgoira()
				if err != nil {
					logrus.Warnln(err)
					ms = createPixivEmbed(post, ind, easterEgg)
				} else {
					a.HasUgoira = true
					ms = createUgoiraEmbed(post, easterEgg)
				}
			} else {
				ms = createPixivEmbed(post, ind, easterEgg)
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

	if a.IsCrosspost {
		for _, m := range messages {
			if strings.Contains(m.Embed.Title, "Page 1") || !strings.Contains(m.Embed.Title, "Page") {
				m.Content = fmt.Sprintf("<%v>", m.Embed.URL)
			}
			m.Embed.Author = &discordgo.MessageEmbedAuthor{Name: fmt.Sprintf("Crosspost requested by %v", a.event.Author.String()), IconURL: a.event.Author.AvatarURL("")}
		}
	}

	return messages
}

func createPixivEmbed(post *ugoira.PixivPost, ind int, easter *embedMessage) *discordgo.MessageSend {
	title := ""

	if post.Len() == 1 {
		title = fmt.Sprintf("%v by %v", post.Title, post.Author)
	} else {
		title = fmt.Sprintf("%v by %v. Page %v/%v", post.Title, post.Author, ind+1, post.Len())
	}

	var (
		original = post.Images.Original[ind].PixivCatProxy
		preview  = post.Images.Preview[ind].PixivCatProxy
	)

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
				URL: preview,
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: easter.Content,
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

func createUgoiraEmbed(post *ugoira.PixivPost, easter *embedMessage) *discordgo.MessageSend {
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
				Text: easter.Content,
			},
		},
	}

	send.Files = append(send.Files, &discordgo.File{
		Name:   fmt.Sprintf("%v.mp4", post.ID),
		Reader: post.Ugoira.File,
	})
	return send
}
