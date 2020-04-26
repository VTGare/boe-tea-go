package commands

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/services"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
)

var (
	//ImageURLRegex is a regex for image URLs
	ImageURLRegex = regexp.MustCompile(`(http(s?):)([/|.|\w|\s|-])*\.(?:jpg|jpeg|gif|png|webp)`)
	searchEngines = map[string]func(link string) (*discordgo.MessageEmbed, error){
		"saucenao": func(link string) (*discordgo.MessageEmbed, error) {
			saucenao, err := services.SearchSauceByURL(link)
			if err != nil {
				return nil, err
			}

			if saucenao.Header.ResultsReturned == 0 {
				return nil, errors.New("no sauce, just ketchup")
			}

			res := (*saucenao.Results)[0]
			author := utils.FindAuthor(res)

			embed := &discordgo.MessageEmbed{
				Title:     "Sauce",
				URL:       res.Data.URLs[0],
				Timestamp: utils.EmbedTimestamp(),
				Color:     utils.EmbedColor,
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
			return embed, nil
		},
		"ascii2d": func(link string) (*discordgo.MessageEmbed, error) {
			res, err := services.GetSauceA2D(link)
			if err != nil {
				return nil, err
			}

			if len(res) == 0 {
				return nil, errors.New("no sauce, just ketchup")
			}

			embed := &discordgo.MessageEmbed{
				Title: fmt.Sprintf("%v by %v on %v", res[0].Name, res[0].Author, res[0].From),
				URL:   res[0].URL,
				Thumbnail: &discordgo.MessageEmbedThumbnail{
					URL: res[0].Thumbnail,
				},
				Color:     utils.EmbedColor,
				Timestamp: utils.EmbedTimestamp(),
				Fields: []*discordgo.MessageEmbedField{
					{
						Name:  "Name",
						Value: res[0].Name,
					},
					{
						Name:  "URL",
						Value: res[0].URL,
					},
					{
						Name:  "Author",
						Value: res[0].Author,
					},
					{
						Name:  "Author URL",
						Value: res[0].AuthorURL,
					},
				},
			}
			return embed, nil
		},
	}
)

func init() {
	Commands["sauce"] = Command{
		Name:            "sauce",
		Description:     "Finds sauce of an anime picture on SauceNAO or ascii2d.",
		GuildOnly:       false,
		Exec:            sauce,
		Help:            true,
		AdvancedCommand: true,
		ExtendedHelp: []*discordgo.MessageEmbedField{
			{
				Name:  "Usage",
				Value: "bt!sauce <search engine> <image link>",
			},
			{
				Name:  "Reverse image search engine",
				Value: "Not required. ``saucenao`` or ``ascii2d``. If omitted uses server's default option",
			},
			{
				Name:  "image link",
				Value: "Required. Link must have one of the following suffixes:  *jpg*, *jpeg*, *png*, *gif*, *webp*.\nURL parameters after the link are accepted (e.g. <link>.jpg***?width=441&height=441***)",
			},
		},
	}
}

func sauce(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(m.Attachments) > 0 {
		args = append(args, m.Attachments[0].URL)
	}

	url := ""
	searchEngine := ""
	switch len(args) {
	case 0:
		return utils.ErrorNotEnoughArguments
	case 1:
		searchEngine = database.GuildCache[m.GuildID].ReverseSearch
		url = ImageURLRegex.FindString(args[0])
		if url == "" {
			return errors.New("received a non-image url")
		}
	case 2:
		if f := ImageURLRegex.FindString(args[0]); f != "" {
			searchEngine = database.GuildCache[m.GuildID].ReverseSearch
			url = f
		} else {
			searchEngine = args[0]
			url = ImageURLRegex.FindString(args[1])
		}

		if url == "" {
			return errors.New("received a non-image url")
		}
	}

	embed, err := searchEngines[searchEngine](url)
	if err != nil {
		return err
	}

	s.ChannelMessageSendEmbed(m.ChannelID, embed)
	return nil
}
