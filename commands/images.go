package commands

import (
	"errors"
	"regexp"

	"github.com/VTGare/boe-tea-go/services"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
)

var (
	ImageURLRegex = regexp.MustCompile(`(http(s?):)([/|.|\w|\s|-])*\.(?:jpg|gif|png|webp)`)
)

func init() {
	Commands["pixiv"] = Command{
		Name:        "pixiv",
		Description: "",
		GuildOnly:   false,
		Exec:        pixiv,
	}

	Commands["sauce"] = Command{
		Name:        "sauce",
		Description: "",
		GuildOnly:   false,
		Exec:        sauce,
	}
}

func pixiv(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	return nil
}

func sauce(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) == 0 {
		return nil
	}

	url := ImageURLRegex.FindString(args[0])
	if url == "" {
		return errors.New("received a non-image url")
	}

	saucenao, err := services.SearchByURL(url)
	if err != nil {
		return err
	}

	if saucenao.Header.ResultsReturned == 0 {
		return errors.New("no sauce, just ketchup")
	}

	res := (*saucenao.Results)[0]
	author := utils.FindAuthor(res)

	embed := &discordgo.MessageEmbed{
		Title: "Sauce",
		URL:   res.Data.URLs[0],
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: res.Header.Thumbnail,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "URL",
				Value: res.Data.URLs[0],
			},
			{
				Name:  "Similarity",
				Value: res.Header.Similarity,
			},
			{
				Name:  "Author",
				Value: author,
			},
		},
	}

	s.ChannelMessageSendEmbed(m.ChannelID, embed)

	return nil
}
