package pixivhelper

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/services"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	//EmbedCache caches sent embeds so users can delete them within certain time interval
	EmbedCache   = make(map[string]string)
	embedWarning = []string{"If you're reading this you're epic.", "If you react ❌ to a pixiv embed it'll be removed", "bt!nhentai 271920, enjoy", "This embed was sponsored by Raid Shadow Legends", "There are several hidden meme commands, try to find them", "Love, from Shamoki-chan", "bt!twitter is useful for mobile users", "Ramiel best girl", "#BlueLivesMatter", "PM the creator of this bot lolis.", "If you wrap a link in <> Discord won't embed it", "Who's Rem", "Every 60 seconds one minute passes in Africa", "People die when they're killed", "You thought it was a useful message, but it was me DIO!", "Enable strict mode to remove filthy reposts."}
)

//Options is a settings structure for configuring Pixiv repost feature for different purposes
type Options struct {
	ProcPrompt bool
	Indexes    map[int]bool
	Exclude    bool
}

//PostPixiv reposts Pixiv images from a link to a discord channel
func PostPixiv(s *discordgo.Session, m *discordgo.MessageCreate, pixivIDs []string, opts ...Options) error {
	if opts == nil {
		opts = []Options{
			{
				ProcPrompt: true,
				Indexes:    map[int]bool{},
				Exclude:    true,
			},
		}
	}

	var (
		guild      = database.GuildCache[m.GuildID]
		posts, err = createPosts(s, m, pixivIDs, opts[0].Indexes)
		flag       = true
	)

	if err != nil {
		return err
	}

	if opts[0].ProcPrompt {
		if len(posts) >= guild.LargeSet {
			message := ""
			if len(posts) >= 3 {
				message = fmt.Sprintf("Large set of images (%v), do you want me to send each image individually?", len(posts))
			} else {
				message = "Do you want me to send each image individually?"
			}

			prompt := utils.CreatePrompt(s, m, &utils.PromptOptions{
				Message: message,
				Actions: map[string]utils.ActionFunc{
					"👌": func() bool {
						return true
					},
				},
				Timeout: time.Second * 15,
			})
			if prompt == nil {
				return nil
			}
			flag = prompt()
		}
	}

	if flag {
		log.Infoln(fmt.Sprintf("Reposting %v images. Guild: %v. Channel: %v", len(posts), guild.GuildID, m.ChannelID))
		postIDs := make([]string, 0)

		for _, message := range posts {
			post, err := s.ChannelMessageSendComplex(m.ChannelID, &message)
			if err != nil {
				return errors.New("a post you're trying to repost is either removed or restricted")
			}

			if post != nil {
				postIDs = append(postIDs, post.ID)
				EmbedCache[post.ID] = m.Author.ID
			}
		}

		go func() {
			time.Sleep(150 * time.Second)

			for _, id := range postIDs {
				delete(EmbedCache, id)
			}
		}()
	}
	return nil
}

func createPosts(s *discordgo.Session, m *discordgo.MessageCreate, pixivIDs []string, excluded map[int]bool) ([]discordgo.MessageSend, error) {
	log.Infoln("Creating posts for following IDs: ", pixivIDs)

	var (
		messages      = make([]discordgo.MessageSend, 0)
		repostSetting = database.GuildCache[m.GuildID].Repost
		ch, _         = s.Channel(m.ChannelID)
		pixivPosts    = make([]*services.PixivPost, 0)
		guild         = database.GuildCache[m.GuildID]
		pageCount     int
	)

	for _, id := range pixivIDs {
		repost, err := utils.IsRepost(m.ChannelID, id)
		if err != nil {
			return nil, err
		}

		if (repostSetting == "enabled" || repostSetting == "strict") && repost != nil {
			if repostSetting == "enabled" {
				prompt := utils.CreatePrompt(s, m, &utils.PromptOptions{
					Actions: map[string]func() bool{
						"✅": func() bool {
							return true
						},
						"❎": func() bool {
							return false
						},
					},
					Message: fmt.Sprintf("Pixiv post %v is a repost, react ✅ if you want to post it anyway or ❎ to skip.", id),
					Timeout: 10 * time.Second,
				})

				if prompt == nil {
					continue
				}

				if !prompt() {
					continue
				}
			}
		}

		post, err := services.GetPixivPost(id)
		if err != nil {
			return nil, err
		}

		if post.NSFW && !ch.NSFW {
			prompt := utils.CreatePrompt(s, m, &utils.PromptOptions{
				Actions: map[string]func() bool{
					"👌": func() bool {
						return true
					},
				},
				Message: "You're trying to repost a post with an R-18 tag, are you sure about that?",
				Timeout: 10 * time.Second,
			})
			if err != nil {
				log.Warnln(err)
				return nil, err
			}
			if prompt == nil {
				continue
			}
		}

		pageCount = post.Pages - len(excluded)
		pixivPosts = append(pixivPosts, post)
	}

	createdCount := 0
	for _, post := range pixivPosts {
		created := false
		if createdCount >= guild.Limit {
			break
		}

		for ind, thumbnail := range post.LargeImages {
			if pageCount > guild.Limit && created {
				break
			}
			if _, ok := excluded[ind+1]; ok {
				continue
			}

			created = true
			createdCount++

			easterEgg := rand.Intn(len(embedWarning))
			utils.NewRepostDetection(m.Author.Username, m.GuildID, m.ChannelID, m.ID, post.ID)
			messages = append(messages, createEmbed(post, thumbnail, post.OriginalImages[ind], ind, easterEgg))
		}
	}

	if pageCount > guild.Limit {
		messages[0].Content = fmt.Sprintf("```Album size (%v) is larger than limit set on this server (%v), only first image of every post is reposted.```", pageCount, guild.Limit)
	}

	return messages, nil
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

func createEmbed(post *services.PixivPost, thumbnail, original string, ind, easterEgg int) discordgo.MessageSend {
	title := ""
	if len(post.LargeImages) == 1 {
		title = fmt.Sprintf("%v by %v", post.Title, post.Author)
	} else {
		title = fmt.Sprintf("%v by %v. Page %v/%v", post.Title, post.Author, ind+1, len(post.LargeImages))
	}

	return discordgo.MessageSend{
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
}
