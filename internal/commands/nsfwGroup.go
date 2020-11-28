package commands

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/pkg/nozoki"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
)

var (
	nh = nozoki.NewNozoki()
)

func init() {
	nsfwG := Router.AddGroup(&gumi.Group{
		Name:        "nsfw",
		Description: "Exquisite commands for real men of culture.",
		NSFW:        true,
		IsVisible:   true,
	})

	nhCmd := nsfwG.AddCommand(&gumi.Command{
		Name:        "nhentai",
		Description: "Sends a detailed description of an nhentai entry",
		Aliases:     []string{"nh"},
		Exec:        nhentai,
		Cooldown:    5 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})
	nhCmd.Help.ExtendedHelp = []*discordgo.MessageEmbedField{
		{
			Name:  "Usage",
			Value: "bt!nhentai <magic number>",
		},
		{
			Name:  "magic number",
			Value: "Typically, but not always, a 6-digit number only weebs understand.",
		},
	}
}

func nhentai(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	var (
		eb = embeds.NewBuilder()
	)

	if g, ok := database.GuildCache[m.GuildID]; ok {
		if !g.NSFW {

			s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
				Title:     "❎ Failed to execute a command.",
				Color:     utils.EmbedColor,
				Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
				Timestamp: utils.EmbedTimestamp(),
				Fields:    []*discordgo.MessageEmbedField{{Name: "Reason", Value: "You're trying to execute an NSFW command. The server prohibits NSFW content."}},
			})
			return nil
		}
	}

	if len(args) == 0 {
		eb.FailureTemplate("``bt!nhentai`` requires an nhentai ID argument.")
		s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
		return nil
	}

	if _, err := strconv.Atoi(args[0]); err != nil {
		return errors.New("invalid nhentai ID")
	}

	book, err := nh.GetBook(args[0])
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
