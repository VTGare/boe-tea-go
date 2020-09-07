package commands

import (
	"errors"
	"fmt"
	"image"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/images"
	"github.com/VTGare/boe-tea-go/internal/repost"
	"github.com/VTGare/boe-tea-go/pkg/chotto"
	"github.com/VTGare/boe-tea-go/pkg/seieki"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	//ImageURLRegex is a regex for image URLs
	ImageURLRegex = regexp.MustCompile(`(?i)(http(s?):)([/|.|\w|\s|-])*\.(?:jpg|jpeg|gif|png|webp)`)
	//ErrNoSauce is an error when source couldn't be found.
	ErrNoSauce       = errors.New("seems like sauce couldn't be found. Try using following websites yourself:\nhttps://ascii2d.net/\nhttps://iqdb.org/")
	messageLinkRegex = regexp.MustCompile(`(?i)http(?:s)?:\/\/(?:www\.)?discord(?:app)?.com\/channels\/\d+\/(\d+)\/(\d+)`)
	sei              = seieki.NewSeieki(os.Getenv("SAUCENAO_API"))
	searchEngines    = map[string]func(link string) (*discordgo.MessageEmbed, error){
		"saucenao": saucenao,
		"wait":     wait,
	}
)

func init() {
	ig := CommandFramework.AddGroup("images", gumi.GroupDescription("Main bot's functionality, source image commands and magick commands"))
	sauceCmd := ig.AddCommand("sauce", sauce, gumi.CommandDescription("Tries to find sauce of an anime picture."))
	sauceCmd.Help.ExtendedHelp = []*discordgo.MessageEmbedField{
		{
			Name:  "Usage",
			Value: "bt!sauce <search engine> <image link>",
		},
		{
			Name:  "Reverse image search engine",
			Value: "Not required. ``saucenao`` or ``wait``. If omitted uses server's default option",
		},
		{
			Name:  "image link",
			Value: "Required. Link must have one of the following suffixes:  *jpg*, *jpeg*, *png*, *gif*, *webp*.\nURL parameters after the link are acceptable (e.g. <link>.jpg***?width=441&height=441***)",
		},
	}

	pixivCmd := ig.AddCommand("pixiv", pixiv, gumi.CommandDescription("Advanced pixiv repost command that lets you exclude images from an album"))
	pixivCmd.Help = gumi.NewHelpSettings().AddField("Usage", "bt!pixiv <post link> [optional excluded images]", false).AddField("excluded images", "Indexes must be separated by whitespace (e.g. 1 2 4 6 10 45)", false)

	dfCmd := ig.AddCommand("deepfry", deepfry, gumi.CommandDescription("Deepfries an image, itadakimasu"))
	dfCmd.Help = gumi.NewHelpSettings()
	dfCmd.Help.AddField("Usage", "bt!deepfry <optional times deepfried> <image link>", false)
	dfCmd.Help.AddField("times deepfried", "Repeats deepfrying process given amount of times, up to 5.", false)

	tCmd := ig.AddCommand("twitter", twitter, gumi.CommandDescription("Reposts each twitter post's image separately. Useful for mobile."))
	tCmd.Help = gumi.NewHelpSettings()
	tCmd.Help.AddField("Usage", "bt!twitter <twitter link>", false)
	tCmd.Help.AddField("Twitter link", "Must look something like this: https://twitter.com/mhy_shima/status/1258684420011069442", false)

	jpegCmd := ig.AddCommand("jpeg", jpegify, gumi.CommandDescription("Gives image a soul. Extremely redpilled command."))
	jpegCmd.Help = gumi.NewHelpSettings()
	jpegCmd.Help.AddField("Usage", "bt!jpeg <image quality> <image url>", false).AddField("image quality", "Optional integer from 0 to 100", false).AddField("image url", "Optional if attachment is present. Attachment is prioritized.", false)
}

func sauce(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	var (
		engine = "saucenao"
		link   = ""
	)

	if guild, ok := database.GuildCache[m.GuildID]; ok {
		engine = guild.ReverseSearch
	}

	if len(args) == 0 || len(args) == 1 && args[0] == "saucenao" || args[0] == "wait" {
		if len(m.Attachments) > 0 && ImageURLRegex.MatchString(m.Attachments[0].URL) {
			args = append(args, m.Attachments[0].URL)
		} else {
			messages, err := s.ChannelMessages(m.ChannelID, 5, m.ID, "", "")
			if err != nil {
				return err
			}
			if recent := findRecentImage(messages); recent != "" {
				args = append(args, recent)
			} else {
				return utils.ErrNotEnoughArguments
			}
		}
	}

	switch {
	case len(args) == 1:
		if arg := ImageURLRegex.FindString(args[0]); arg != "" {
			link = arg
		} else {
			str, err := sauceInMessageLink(s, args[0])
			if err != nil {
				return err
			}
			link = str
		}

		if link == "" {
			return fmt.Errorf("incorrect command usage. Please refer to ``bt!help images sauce`` for more information")
		}
	case len(args) >= 2:
		if ImageURLRegex.MatchString(args[0]) {
			link = args[0]
		} else if args[0] == "saucenao" || args[0] == "wait" {
			engine = args[0]
			if ImageURLRegex.MatchString(args[1]) {
				link = args[1]
			} else if messageLinkRegex.MatchString(args[1]) {
				str, err := sauceInMessageLink(s, args[1])
				if err != nil {
					return err
				}
				if str != "" {
					link = str
				}
			}
		}
		if link == "" {
			return fmt.Errorf("incorrect command usage. Please refer to ``bt!help images sauce`` for more information")
		}
	}

	log.Infof("Searching for source image. URL: %v. Reverse search engine: %v", link, engine)
	embed, err := searchEngines[engine](link)
	if err != nil {
		return err
	}
	_, err = s.ChannelMessageSendEmbed(m.ChannelID, embed)
	if err != nil {
		return err
	}

	return nil
}

func sauceInMessageLink(s *discordgo.Session, arg string) (string, error) {
	if matches := messageLinkRegex.FindStringSubmatch(arg); matches != nil {
		m, err := s.ChannelMessage(matches[1], matches[2])
		if err != nil {
			return "", err
		}
		if recent := findRecentImage([]*discordgo.Message{m}); recent != "" {
			return recent, nil
		}
	}

	return "", nil
}

func namedLink(uri string) string {
	switch {
	case strings.Contains(uri, "danbooru"):
		return fmt.Sprintf("[Danbooru](%v)", uri)
	case strings.Contains(uri, "gelbooru"):
		return fmt.Sprintf("[Gelbooru](%v)", uri)
	case strings.Contains(uri, "sankakucomplex"):
		return fmt.Sprintf("[Sankakucomplex](%v)", uri)
	case strings.Contains(uri, "pixiv"):
		return fmt.Sprintf("[Pixiv](%v)", uri)
	case strings.Contains(uri, "twitter"):
		return fmt.Sprintf("[Twitter](%v)", uri)
	default:
		return uri
	}
}

func joinSauceURLs(urls []string, sep string) string {
	var sb strings.Builder
	if len(urls) == 0 {
		return "-"
	}

	sb.WriteString(namedLink(urls[0]))
	for _, uri := range urls[1:] {
		sb.WriteString(sep)
		sb.WriteString(namedLink(uri))
	}

	return sb.String()
}

func saucenao(link string) (*discordgo.MessageEmbed, error) {
	res, err := sei.Sauce(link)
	if err != nil && res == nil {
		return nil, err
	}

	res.FilterLowSimilarity(60.0)
	if len(res.Results) == 0 {
		return nil, ErrNoSauce
	}

	source := res.Results[0]
	log.Infof("Found source. Author: %v. Title: %v. URL: %v", source.Author(), source.Title(), source.URL())

	embed := &discordgo.MessageEmbed{
		Title:     fmt.Sprintf("Source found! Title: %v", source.Title()),
		Timestamp: utils.EmbedTimestamp(),
		Color:     utils.EmbedColor,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: source.Header.Thumbnail,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Source",
				Value: source.URL(),
			},
			{
				Name:  "URLs",
				Value: joinSauceURLs(source.Data.URLs, " • "),
			},
			{
				Name:  "Similarity",
				Value: source.Header.Similarity,
			},
			{
				Name:  "Author",
				Value: source.Author(),
			},
		},
	}

	if s := source.URL(); s != "" {
		if _, err := url.ParseRequestURI(embed.URL); err != nil && len(source.Data.URLs) > 0 {
			embed.URL = source.Data.URLs[0]
		}
	} else {
		embed.Fields = embed.Fields[1:]
	}

	return embed, nil
}

func wait(link string) (*discordgo.MessageEmbed, error) {
	res, err := chotto.SearchWait(link)
	if err != nil {
		return nil, err
	}

	if len(res.Documents) == 0 {
		return nil, errors.New("couldn't find source anime")
	}

	anime := res.Documents[0]

	description := ""
	uri := ""
	if anime.AnilistID != 0 && anime.MalID != 0 {
		description = fmt.Sprintf("[AniList link](https://anilist.co/anime/%v/) | [MyAnimeList link](https://myanimelist.net/anime/%v/)", anime.AnilistID, anime.MalID)
		uri = fmt.Sprintf("https://myanimelist.net/anime/%v/", anime.MalID)
	} else if anime.AnilistID != 0 {
		description = fmt.Sprintf("[AniList link](https://anilist.co/anime/%v/)", anime.AnilistID)
		uri = fmt.Sprintf("https://anilist.co/anime/%v/", anime.AnilistID)
	} else if anime.MalID != 0 {
		description = fmt.Sprintf("[MyAnimeList link](https://myanimelist.net/anime/%v/)", anime.MalID)
		uri = fmt.Sprintf("https://myanimelist.net/anime/%v/", anime.MalID)
	} else {
		description = "No links :shrug:"
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%v | %v", anime.TitleEnglish, anime.TitleNative),
		URL:         uri,
		Description: description,
		Color:       utils.EmbedColor,
		Timestamp:   utils.EmbedTimestamp(),
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "Similarity",
				Value: fmt.Sprintf("%v%%", anime.Similarity*100),
			},
			{
				Name:  "Timestamp",
				Value: fmt.Sprintf("%v", readableSeconds(anime.At)),
			},
			{
				Name:  "Episode",
				Value: fmt.Sprintf("%v", anime.Episode),
			},
		},
	}

	return embed, nil
}

func readableSeconds(sec float64) string {
	return fmt.Sprintf("%v:%v", int(sec)/60, int(sec)%60)
}

func findRecentImage(messages []*discordgo.Message) string {
	for _, msg := range messages {
		f := ImageURLRegex.FindString(msg.Content)
		switch {
		case f != "":
			return f
		case len(msg.Attachments) > 0:
			return msg.Attachments[0].URL
		case len(msg.Embeds) > 0:
			if msg.Embeds[0].Image != nil {
				return msg.Embeds[0].Image.URL
			}
		}
	}

	return ""
}

func pixiv(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}
	guild := database.GuildCache[m.GuildID]

	rep := repost.NewPost(*m, false, args[0])
	if rep.Len() == 0 {
		return errors.New("first arguments must be a pixiv link")
	}
	if guild.Repost == "strict" {
		rep.FindReposts()
		if len(rep.Reposts) != 0 {
			_, err := s.ChannelMessageSendEmbed(m.ChannelID, rep.RepostEmbed())
			if err != nil {
				log.Warnln(err)
			}
			return nil
		}
	}

	args = args[1:]
	excludes := make(map[int]bool)
	for _, arg := range args {
		if strings.Contains(arg, "-") {
			ran, err := utils.NewRange(arg)
			if err != nil {
				return err
			}

			for i := ran.Low; i <= ran.High; i++ {
				excludes[i] = true
			}
		} else {
			num, err := strconv.Atoi(arg)
			if err != nil {
				return utils.ErrParsingArgument
			}
			excludes[num] = true
		}
	}

	messages, err := rep.SendPixiv(s, repost.SendPixivOptions{
		Exclude: excludes,
	})
	if err != nil {
		return err
	}

	for _, mes := range messages {
		s.ChannelMessageSendComplex(m.ChannelID, mes)
	}

	if rep.HasUgoira {
		rep.Cleanup()
	}
	return nil
}

func twitter(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	guild := database.GuildCache[m.GuildID]
	a := repost.NewPost(*m, false, args[0])

	if guild.Repost != "disabled" {
		a.FindReposts()
		if len(a.Reposts) > 0 {
			switch guild.Repost {
			case "strict":
				s.ChannelMessageSendEmbed(m.ChannelID, a.RepostEmbed())
				return nil
			case "enabled":
				f := utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
					Content: "Tweet you're trying to post is a repost. Are you sure about that?",
					Embed:   a.RepostEmbed(),
				})
				if !f {
					return nil
				}
			}
		}
	}

	tweets, err := a.SendTwitter(s, false)
	if err != nil {
		return err
	}

	for _, t := range tweets {
		for _, send := range t {
			_, err := s.ChannelMessageSendComplex(m.ChannelID, send)
			if err != nil {
				log.Warnln(err)
			}
		}
	}

	return nil
}

func deepfry(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(m.Attachments) > 0 {
		args = append(args, m.Attachments[0].URL)
	}

	uri := ""
	times := 0
	switch len(args) {
	case 2:
		if f := ImageURLRegex.FindString(args[0]); f != "" {
			uri = f
		} else {
			var err error
			times, err = strconv.Atoi(args[0])
			if times > 5 {
				return errors.New("can't deepfry more than 5 times at once")
			}
			if err != nil {
				return err
			}
			uri = ImageURLRegex.FindString(args[1])
		}

		if uri == "" {
			return errors.New("received a non-image url")
		}
	case 1:
		if f := ImageURLRegex.FindString(args[0]); f != "" {
			uri = f
		} else {
			return errors.New("received a non-image url")
		}
	case 0:
		return utils.ErrNotEnoughArguments
	}

	img, err := images.DownloadImage(uri)
	if err != nil {
		return err
	}

	deepfried := images.Deepfry(img)
	for i := 0; i < times; i++ {
		img, _, _ := image.Decode(deepfried)
		deepfried = images.Deepfry(img)
	}

	s.ChannelFileSend(m.ChannelID, "deepfried.jpg", deepfried)
	return nil
}

func jpegify(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(m.Attachments) > 0 {
		args = append(args, m.Attachments[0].URL)
	}

	uri := ""
	quality := 10
	switch len(args) {
	case 2:
		if f := ImageURLRegex.FindString(args[0]); f != "" {
			uri = f
		} else {
			var err error
			quality, err = strconv.Atoi(args[0])
			if quality > 100 || quality < 1 {
				return errors.New("quality can't be higher than 100 or lower than 1")
			}
			if err != nil {
				return err
			}
			uri = ImageURLRegex.FindString(args[1])
		}

		if uri == "" {
			return errors.New("received a non-image url")
		}
	case 1:
		if f := ImageURLRegex.FindString(args[0]); f != "" {
			uri = f
		} else {
			return errors.New("received a non-image url")
		}
	case 0:
		return utils.ErrNotEnoughArguments
	}

	img, err := images.DownloadImage(uri)
	if err != nil {
		return err
	}

	deepfried := images.Jpegify(img, quality)
	s.ChannelFileSend(m.ChannelID, "soul.jpg", deepfried)
	return nil
}
