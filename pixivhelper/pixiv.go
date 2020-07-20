package pixivhelper

import (
	"fmt"
	"regexp"
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
	//Regex is a regular experession that detects various Pixiv links
	Regex = regexp.MustCompile(`(?i)http(?:s)?:\/\/(?:www\.)?pixiv\.net\/(?:en\/)?(?:artworks\/|member_illust\.php\?)(?:mode=medium\&)?(?:illust_id=)?([0-9]+)`)
	//EmbedCache caches sent embeds so users can delete them within certain time interval
	EmbedCache   = make(map[string]string)
	embedWarning = fmt.Sprintf("Please follow the link in the title to download high-res image")
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
			post, _ := s.ChannelMessageSendComplex(m.ChannelID, &message)
			postIDs = append(postIDs, post.ID)
			EmbedCache[post.ID] = m.Author.ID
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
		strictIDs     = make([]string, 0)
		ch, _         = s.Channel(m.ChannelID)
		pixivPosts    = make([]*services.PixivPost, 0)
		guild         = database.GuildCache[m.GuildID]
		pageCount     int
	)

	for _, id := range pixivIDs {
		if (repostSetting == "enabled" || repostSetting == "strict") && utils.IsRepost(m.ChannelID, id) {
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
			} else if repostSetting == "strict" {
				strictIDs = append(strictIDs, id)
				continue
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

			utils.NewRepostChecker(m.ChannelID, post.ID)
			messages = append(messages, createEmbed(post, thumbnail, post.OriginalImages[ind], ind))
		}
	}

	if pageCount > guild.Limit {
		messages[0].Content = fmt.Sprintf("```Album size (%v) is larger than limit set on this server (%v), only first image of every post is reposted.```", pageCount, guild.Limit)
	}

	if repostSetting == "strict" && len(strictIDs) > 0 {
		if len(strictIDs) == len(pixivIDs) {
			if f, _ := utils.MemberHasPermission(s, m.GuildID, s.State.User.ID, discordgo.PermissionManageMessages|discordgo.PermissionAdministrator); f {
				err := s.ChannelMessageDelete(m.ChannelID, m.ID)
				if err != nil {
					log.Warn(err)
				}
			}
		}

		content := ""
		if len(strictIDs) == 1 {
			content = fmt.Sprintf("Pixiv post ``%v`` is a repost and has been skipped.", strictIDs)
		} else {
			content = fmt.Sprintf("Pixiv posts with following IDs ``%v`` are reposts and have been skipped.", strictIDs)
		}

		msg, _ := s.ChannelMessageSend(m.ChannelID, content)
		go func() {
			time.Sleep(15 * time.Second)
			s.ChannelMessageDelete(msg.ChannelID, msg.ID)
		}()
	}
	return messages, nil
}

func createEmbed(post *services.PixivPost, thumbnail, original string, ind int) discordgo.MessageSend {
	title := ""
	if len(post.LargeImages) == 1 {
		title = fmt.Sprintf("%v by %v", post.Title, post.Author)
	} else {
		title = fmt.Sprintf("%v by %v. Page %v/%v", post.Title, post.Author, ind+1, len(post.LargeImages))
	}

	return discordgo.MessageSend{
		Embed: &discordgo.MessageEmbed{
			Title:     title,
			URL:       original,
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
					Value:  strings.Join(post.Tags, " • "),
					Inline: true,
				},
			},
			Image: &discordgo.MessageEmbedImage{
				URL: thumbnail,
			},
			Footer: &discordgo.MessageEmbedFooter{
				Text: embedWarning,
			},
		},
	}
}
