package utils

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/services"
	"github.com/bwmarrin/discordgo"
)

type ActionFunc = func() bool

type PromptOptions struct {
	Actions map[string]ActionFunc
	Message string
	Timeout time.Duration
}

var (
	r = regexp.MustCompile(`http(?:s)?:\/\/(?:www\.)?pixiv\.net\/(?:en\/)?artworks\/([0-9]+)`)
)

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

func PostPixiv(s *discordgo.Session, m *discordgo.MessageCreate, text string) {
	matches := r.FindStringSubmatch(text)

	if matches == nil {
		return
	}

	images, err := services.GetPixivImages(matches[1])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Error: "+err.Error())
		return
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
			return
		}
		flag = prompt()
	}

	if flag {
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
			return
		}
		links := prompt()

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
}

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
			return nil
		}

		if _, ok := opts.Actions[reaction.Emoji.Name]; !ok {
			continue
		}

		if reaction.MessageID != prompt.ID || s.State.User.ID == reaction.UserID || reaction.UserID != m.Author.ID {
			continue
		}

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
