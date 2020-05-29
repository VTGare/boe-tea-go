package utils

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/services"
	"github.com/bwmarrin/discordgo"
)

var (
	PixivRegex = regexp.MustCompile(`http(?:s)?:\/\/(?:www\.)?pixiv\.net\/(?:en\/)?(?:artworks\/|member_illust\.php\?)(?:mode=medium\&)?(?:illust_id=)?([0-9]+)`)
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

	var ask bool
	var links bool
	guild := database.GuildCache[m.GuildID]

	switch guild.Repost {
	case "ask":
		ask = true
	case "links":
		ask = false
		links = true
	case "embeds":
		ask = false
		links = false
	}

	if ask {
		prompt := CreatePrompt(s, m, &PromptOptions{
			Message: "Send images as links (✅) or embeds (❎)?",
			Actions: map[string]ActionFunc{
				"✅": func() bool {
					return true
				},
				"❎": func() bool {
					return false
				},
			},
			Timeout: time.Second * 15,
		})
		if prompt == nil {
			return nil
		}
		links = prompt()
	}

	messages, err := createPosts(s, m.ChannelID, pixivIDs, links)
	if err != nil {
		return err
	}

	flag := true
	if opts[0].ProcPrompt {
		if len(messages) >= guild.LargeSet {
			message := ""
			if len(messages) >= 3 {
				message = fmt.Sprintf("Large set of images (%v), do you want me to send each image individually?", len(messages))
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
		log.Println(fmt.Sprintf("Successfully reposting %v images in %v", len(messages), guild.GuildID))

		postIDs := make([]string, 0)

		if len(messages) > guild.Limit {
			messages[0].Content = fmt.Sprintf("```Album size (%v) is off limits (%v), only first image is reposted.```", len(messages), guild.Limit)

			post, _ := s.ChannelMessageSendComplex(m.ChannelID, &messages[0])
			postIDs = append(postIDs, post.ID)
			PostCache[post.ID] = m.Author.ID
			return nil
		}

		for ind, message := range messages {
			if _, ok := opts[0].Indexes[ind+1]; ok {
				continue
			}

			post, _ := s.ChannelMessageSendComplex(m.ChannelID, &message)
			postIDs = append(postIDs, post.ID)
			PostCache[post.ID] = m.Author.ID
		}

		go func() {
			time.Sleep(30 * time.Second)

			for _, id := range postIDs {
				delete(PostCache, id)
			}
		}()
	}
	return nil
}

func createPosts(s *discordgo.Session, channelID string, pixivIDs []string, links bool) ([]discordgo.MessageSend, error) {
	messages := make([]discordgo.MessageSend, 0)

	ch, _ := s.Channel(channelID)
	for _, link := range pixivIDs {
		post, err := services.GetPixivPost(link)
		if err != nil {
			return nil, err
		}

		if post.NSFW && !ch.NSFW {
			s.ChannelMessageSend(channelID, "⚠ Can't repost NSFW post in SFW channel.")
			continue
		}

		for ind, image := range post.LargeImages {
			title := ""
			if len(post.LargeImages) == 1 {
				title = fmt.Sprintf("%v by %v", post.Title, post.Author)
			} else {
				title = fmt.Sprintf("%v by %v. Page %v/%v", post.Title, post.Author, ind+1, len(post.LargeImages))
			}

			if links {
				title += fmt.Sprintf("\n%v\n♥ %v", image, post.Likes)
				messages = append(messages, discordgo.MessageSend{
					Content: title,
				})
			} else {
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
	}

	return messages, nil
}
