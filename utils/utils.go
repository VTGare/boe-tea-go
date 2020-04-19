package utils

import (
	"fmt"
	"log"
	"regexp"
	"strconv"
	"time"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/services"
	"github.com/bwmarrin/discordgo"
)

//ActionFunc is a function type alias for prompt actions
type ActionFunc = func() bool

//PromptOptions is a struct that defines prompt's behaviour.
type PromptOptions struct {
	Actions map[string]ActionFunc
	Message string
	Timeout time.Duration
}

var (
	r = regexp.MustCompile(`http(?:s)?:\/\/(?:www\.)?pixiv\.net\/(?:en\/)?artworks\/([0-9]+)`)
)

//FindAuthor is a SauceNAO helper function that finds original source author string.
func FindAuthor(sauce services.Sauce) string {
	if sauce.Data.MemberName != "" {
		return sauce.Data.MemberName
	} else if sauce.Data.Author != "" {
		return sauce.Data.Author
	} else if creator, ok := sauce.Data.Creator.(string); ok {
		return creator
	}

	return "-"
}

//PostPixiv reposts Pixiv images from a link to a discord channel
func PostPixiv(s *discordgo.Session, m *discordgo.MessageCreate, text string) error {
	matches := r.FindStringSubmatch(text)

	if matches == nil {
		return nil
	}

	in := ""
	if m.GuildID != "" {
		g, _ := s.Guild(m.GuildID)
		in = g.Name
	} else {
		in = "DMs"
	}

	log.Println(fmt.Sprintf("Reposting Pixiv images in %v, requested by %v", in, m.Author.String()))
	images, err := services.GetPixivImages(matches[1])
	if err != nil {
		return err
	}

	flag := true
	if len(images) >= database.GuildCache[m.GuildID].LargeSet {
		flag = false
		prompt := CreatePrompt(s, m, &PromptOptions{
			Message: "Large image set (" + strconv.Itoa(len(images)) + "), do you want me to post each picture individually?",
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

	if flag {
		var ask bool
		var links bool
		if g, ok := database.GuildCache[m.GuildID]; ok {
			switch g.RepostAs {
			case "ask":
				ask = true
			case "links":
				ask = false
				links = true
			case "embeds":
				ask = false
				links = false
			}
		}

		if ask {
			prompt := CreatePrompt(s, m, &PromptOptions{
				Message: "Send images as links (✅) or embeds (❎)? ***Warning: embeds sometimes lag!***",
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

		for ind, image := range images {
			if links {
				content := fmt.Sprintf("Image %v/%v\n%v", strconv.Itoa(ind+1), strconv.Itoa(len(images)), image)
				s.ChannelMessageSend(m.ChannelID, content)
			} else {
				title := fmt.Sprintf("Image %v/%v", strconv.Itoa(ind+1), strconv.Itoa(len(images)))
				description := fmt.Sprintf("If embed is empty follow this link to see the image: %v", image)
				embed := &discordgo.MessageEmbed{
					Title:       title,
					Description: description,
					URL:         image,
					Timestamp:   time.Now().Format(time.RFC3339),
				}
				embed.Image = &discordgo.MessageEmbedImage{
					URL: image,
				}

				s.ChannelMessageSendEmbed(m.ChannelID, embed)
			}
		}
	}

	return nil
}

//CreatePrompt sends a prompt message to a discord channel
func CreatePrompt(s *discordgo.Session, m *discordgo.MessageCreate, opts *PromptOptions) ActionFunc {
	prompt, _ := s.ChannelMessageSend(m.ChannelID, opts.Message)
	for emoji := range opts.Actions {
		s.MessageReactionAdd(m.ChannelID, prompt.ID, emoji)
	}

	var reaction *discordgo.MessageReaction
	for {
		select {
		case k := <-nextMessageReactionAdd(s):
			reaction = k.MessageReaction
		case <-time.After(opts.Timeout):
			s.ChannelMessageDelete(prompt.ChannelID, prompt.ID)
			return nil
		}

		if _, ok := opts.Actions[reaction.Emoji.Name]; !ok {
			continue
		}

		if reaction.MessageID != prompt.ID || s.State.User.ID == reaction.UserID || reaction.UserID != m.Author.ID {
			continue
		}

		s.ChannelMessageDelete(prompt.ChannelID, prompt.ID)
		return opts.Actions[reaction.Emoji.Name]
	}
}

func nextMessageReactionAdd(s *discordgo.Session) chan *discordgo.MessageReactionAdd {
	out := make(chan *discordgo.MessageReactionAdd)
	s.AddHandlerOnce(func(_ *discordgo.Session, e *discordgo.MessageReactionAdd) {
		out <- e
	})
	return out
}
