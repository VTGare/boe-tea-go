package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	nhentaiAPI "github.com/VTGare/boe-tea-go/nhentai"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
)

func init() {
	nsfwGroup := CommandGroup{
		Name:        "nsfw",
		Description: "All kinds of potentionally NSFW commands are here",
		NSFW:        true,
		Commands:    make(map[string]Command),
		IsVisible:   true,
	}

	nhentaiCommand := newCommand("nhentai", "Sends detailed information about an nhentai book.").setExec(nhentai).setHelp(&HelpSettings{
		IsVisible: true,
		ExtendedHelp: []*discordgo.MessageEmbedField{
			{
				Name:  "Usage",
				Value: "bt!nhentai <magic number>",
			},
			{
				Name:  "magic number",
				Value: "Typically, but not always, a 6-digit number only weebs understand.",
			},
		},
	})

	nsfwGroup.addCommand(nhentaiCommand)
	CommandGroups["nsfw"] = nsfwGroup
}

func nhentai(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	if _, err := strconv.Atoi(args[0]); err != nil {
		return errors.New("invalid nhentai ID")
	}

	ch, err := s.Channel(m.ChannelID)
	if err != nil {
		return err
	}

	if !ch.NSFW {
		prompt := utils.CreatePrompt(s, m, &utils.PromptOptions{
			Actions: map[string]func() bool{
				"👌": func() bool { return true },
			},
			Message: "Are you sure you want to use ``nhentai`` in an SFW channel? React 👌 to confirm.",
			Timeout: 15 * time.Second,
		})
		if prompt == nil {
			return nil
		}
	}

	book, err := nhentaiAPI.GetNHentai(args[0])
	if err != nil {
		return err
	}

	artists := ""
	tags := ""
	if str := strings.Join(book.Artists, ", "); str != "" {
		artists = str
	} else {
		artists = "-"
	}

	if str := strings.Join(book.Tags, ", "); str != "" {
		tags = str
	} else {
		tags = "-"
	}

	embed := &discordgo.MessageEmbed{
		URL:   book.URL,
		Title: book.Titles.Pretty,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: book.Cover,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Artists",
				Value: artists,
			}, {
				Name:  "Tags",
				Value: tags,
			}, {
				Name:  "Favourites",
				Value: fmt.Sprintf("%v", book.Favourites),
			}, {
				Name:  "Pages",
				Value: fmt.Sprintf("%v", book.Pages),
			},
		},
		Color:     utils.EmbedColor,
		Timestamp: utils.EmbedTimestamp(),
	}

	_, err = s.ChannelMessageSendEmbed(m.ChannelID, embed)
	if err != nil {
		return err
	}

	return nil
}
