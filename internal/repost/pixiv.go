package repost

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"sync"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/embeds"
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
	if !utils.IsPixivUp {
		return nil, nil, errors.New("pixiv api is down")
	}

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
			eb := embeds.NewBuilder().FailureTemplate("An NSFW post has been detected. The server prohibits NSFW content.")
			s.ChannelMessageSendEmbed(a.event.ChannelID, eb.Finalize())

			return nil, nil, nil
		}
		ch, err := s.Channel(a.event.ChannelID)
		if err != nil {
			return nil, nil, err
		}

		if !ch.NSFW {
			eb := embeds.NewBuilder().WarnTemplate("You're trying to send an NSFW post in a non-NSFW channel. Are you sure?")
			prompt := utils.CreatePromptWithMessage(s, a.event, &discordgo.MessageSend{
				Embed: eb.Finalize(),
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
		original = ""
		preview  = ""
	)

	switch database.DevSet.PixivReverseProxy {
	case database.KotoriLove:
		original = post.Images.Original[ind].Kotori
		preview = post.Images.Preview[ind].Kotori
	case database.PixivCat:
		original = post.Images.Original[ind].PixivCat
		preview = post.Images.Preview[ind].PixivCat
	case database.PixivCatProxy:
		original = post.Images.Original[ind].PixivCatProxy
		preview = post.Images.Preview[ind].PixivCatProxy
	}

	eb := embeds.NewBuilder()
	eb.Title(title).URL(post.URL).Image(preview)

	if strings.Contains(easter.Content, "Shit waifu") && post.GoodWaifu {
		eb.Footer("Good taste, m8", "")
	} else {
		eb.Footer(easter.Content, "")
	}

	eb.AddField("Likes", strconv.Itoa(post.Likes), true).AddField("Original quality", fmt.Sprintf("[Click here desu~](%v)", original), true)
	eb.AddField("Liked an artwork?", "Add it to favourites!\nReact: 💖 - as sfw | 🤤 - as nsfw")

	if ind == 0 {
		eb.Description(fmt.Sprintf("**Tags**\n%v", joinTags(post.Tags, " • ")))
	}

	send := &discordgo.MessageSend{Embed: eb.Finalize()}
	return send
}

func createUgoiraEmbed(post *ugoira.PixivPost, easter *embedMessage) *discordgo.MessageSend {
	title := fmt.Sprintf("%v by %v", post.Title, post.Author)

	eb := embeds.NewBuilder()
	eb.Title(title).URL(post.URL).Footer(easter.Content, "")
	eb.AddField("Likes", strconv.Itoa(post.Likes), true)
	eb.AddField("Tags", joinTags(post.Tags, " • "), true)
	send := &discordgo.MessageSend{
		Embed: eb.Finalize(),
	}

	send.Files = append(send.Files, &discordgo.File{
		Name:   fmt.Sprintf("%v.mp4", post.ID),
		Reader: post.Ugoira.File,
	})
	return send
}
