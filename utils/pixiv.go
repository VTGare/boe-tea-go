package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/services"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	PixivRegex = regexp.MustCompile(`(?i)http(?:s)?:\/\/(?:www\.)?pixiv\.net\/(?:en\/)?(?:artworks\/|member_illust\.php\?)(?:mode=medium\&)?(?:illust_id=)?([0-9]+)`)
	PostCache  = make(map[string]string)
)

type PixivOptions struct {
	ProcPrompt bool
	Indexes    map[int]bool
	Exclude    bool
}

//PostPixiv reposts Pixiv images from a link to a discord channel
func PostPixiv(s *discordgo.Session, m *discordgo.MessageCreate, pixivIDs []string, opts ...PixivOptions) error {
	if opts == nil {
		opts = []PixivOptions{
			{
				ProcPrompt: true,
				Indexes:    map[int]bool{},
				Exclude:    true,
			},
		}
	}

	guild := database.GuildCache[m.GuildID]
	posts, err := createPosts(s, m, pixivIDs, opts[0].Indexes)
	if err != nil {
		return err
	}

	flag := true
	if opts[0].ProcPrompt {
		if len(posts) >= guild.LargeSet {
			message := ""
			if len(posts) >= 3 {
				message = fmt.Sprintf("Large set of images (%v), do you want me to send each image individually?", len(posts))
			} else {
				message = "Do you want me to send each image individually?"
			}

			prompt := CreatePrompt(s, m, &PromptOptions{
				Message: message,
				Actions: map[string]ActionFunc{
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
		if len(posts) > guild.Limit {
			posts[0].Content = fmt.Sprintf("```Album size (%v) is larger than limit set on this server (%v), only first image is reposted.```", len(posts), guild.Limit)

			post, _ := s.ChannelMessageSendComplex(m.ChannelID, &posts[0])
			postIDs = append(postIDs, post.ID)
			PostCache[post.ID] = m.Author.ID
			return nil
		}

		for _, message := range posts {
			post, _ := s.ChannelMessageSendComplex(m.ChannelID, &message)
			postIDs = append(postIDs, post.ID)
			PostCache[post.ID] = m.Author.ID
		}

		go func() {
			time.Sleep(150 * time.Second)

			for _, id := range postIDs {
				delete(PostCache, id)
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
	)

	for _, id := range pixivIDs {
		if IsRepost(m.GuildID, id) {
			switch repostSetting {
			case "enabled":
				prompt := CreatePrompt(s, m, &PromptOptions{
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
			case "strict":
				strictIDs = append(strictIDs, id)
				continue
			}
		}
		post, err := services.GetPixivPost(id)
		if err != nil {
			return nil, err
		}
		NewRepostChecker(m.GuildID, id)

		if post.NSFW && !ch.NSFW {
			prompt := CreatePrompt(s, m, &PromptOptions{
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

		for ind, image := range post.LargeImages {
			if _, ok := excluded[ind+1]; ok {
				continue
			}

			title := ""
			if len(post.LargeImages) == 1 {
				title = fmt.Sprintf("%v by %v", post.Title, post.Author)
			} else {
				title = fmt.Sprintf("%v by %v. Page %v/%v", post.Title, post.Author, ind+1, len(post.LargeImages))
			}

			embedWarning := fmt.Sprintf("Please follow the link in the title to download high-res image")
			messages = append(messages, discordgo.MessageSend{
				Embed: &discordgo.MessageEmbed{
					Title:     title,
					URL:       post.OriginalImages[ind],
					Color:     EmbedColor,
					Timestamp: time.Now().Format(time.RFC3339),
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
						URL: image,
					},
					Footer: &discordgo.MessageEmbedFooter{
						Text: embedWarning,
					},
				},
			})
		}
	}

	if repostSetting == "strict" {
		if len(strictIDs) > 1 {
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Pixiv posts with following IDs ``%v`` are reposts and have been skipped.", strictIDs))
		} else if len(strictIDs) == 1 {
			if f, _ := MemberHasPermission(s, m.GuildID, s.State.User.ID, discordgo.PermissionManageMessages|discordgo.PermissionAdministrator); f {
				err := s.ChannelMessageDelete(m.ChannelID, m.ID)
				if err != nil {
					log.Warn(err)
				}
			}
			s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Pixiv post ``%v`` is a repost and has been skipped.", strictIDs))
		}
	}
	return messages, nil
}
