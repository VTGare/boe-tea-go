package commands

import (
	"errors"
	"fmt"
	"image"
	"net/http"
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
	"github.com/VTGare/boe-tea-go/pkg/tsuita"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	//ImageURLRegex is a regex for image URLs
	ImageURLRegex = regexp.MustCompile(`(?i)(http(s?):)([/|.|\w|\s|-])*\.(?:jpg|jpeg|gif|png|webp)`)
	//ErrNoSauce is an error when source couldn't be found.
	ErrNoSauce    = errors.New("seems like sauce couldn't be found. Try using following websites yourself:\nhttps://ascii2d.net/\nhttps://iqdb.org/")
	sei           = seieki.NewSeieki(os.Getenv("SAUCENAO_API"))
	searchEngines = map[string]func(link string) (*discordgo.MessageEmbed, error){
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
	dfCmd.Help = gumi.NewHelpSettings().AddField("Usage", "bt!deepfry <optional times deepfried> <image link>", false).AddField("times deepfried", "Repeats deepfrying process given amount of times, up to 5.", false)
	tCmd := ig.AddCommand("twitter", twitter, gumi.CommandDescription("Reposts each twitter post's image separately. Useful for mobile."))
	tCmd.Help = gumi.NewHelpSettings().AddField("Usage", "bt!twitter <twitter link>", false).AddField("Twitter link", "Must look something like this: https://twitter.com/mhy_shima/status/1258684420011069442", false)
}

func sauce(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(m.Attachments) > 0 {
		args = append(args, m.Attachments[0].URL)
	}

	messages, err := s.ChannelMessages(m.ChannelID, 5, m.ID, "", "")
	if err != nil {
		return err
	}

	if recent := findRecentImage(messages); recent != "" {
		args = append(args, recent)
	}

	findEngine := func() string {
		if m.GuildID != "" {
			return database.GuildCache[m.GuildID].ReverseSearch
		}
		return "saucenao"
	}

	uri := ""
	searchEngine := ""

	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	} else if len(args) == 1 {
		searchEngine = findEngine()
		uri = ImageURLRegex.FindString(args[0])
		if uri == "" {
			return errors.New("received a non-image url")
		}
	} else if len(args) >= 2 {
		if f := ImageURLRegex.FindString(args[0]); f != "" {
			searchEngine = findEngine()
			uri = f
		} else if _, ok := searchEngines[args[0]]; ok {
			searchEngine = args[0]
			uri = ImageURLRegex.FindString(args[1])
		} else {
			return errors.New("incorrect command usage, please use bt!help sauce for more info")
		}

		if uri == "" {
			return errors.New("received a non-image url")
		}
	}

	log.Infof("Searching for source image. URL: %v. Reverse search engine: %v", uri, searchEngine)
	embed, err := searchEngines[searchEngine](uri)
	if err != nil {
		return err
	}
	_, err = s.ChannelMessageSendEmbed(m.ChannelID, embed)
	if err != nil {
		return err
	}

	return nil
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
	log.Infof("Found source. Resulting struct: %v", source)

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

	rep := repost.NewPost(*m, args[0])
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
		SkipPrompt: true,
		Exclude:    excludes,
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

	tweet, err := tsuita.GetTweet(args[0])
	if err != nil {
		return err
	}

	messages := make([]discordgo.MessageSend, 0)
	for ind, media := range tweet.Gallery {
		title := ""
		if len(tweet.Gallery) > 1 {
			title = fmt.Sprintf("%v's tweet | Page %v/%v", tweet.Author, ind+1, len(tweet.Gallery))
		} else {
			title = fmt.Sprintf("%v's tweet", tweet.Author)
		}

		embed := discordgo.MessageEmbed{
			Title:     title,
			Timestamp: tweet.Timestamp,
			Fields: []*discordgo.MessageEmbedField{
				{
					Name:   "Likes",
					Value:  strconv.Itoa(tweet.Likes),
					Inline: true,
				},
				{
					Name:   "Retweets",
					Value:  strconv.Itoa(tweet.Retweets),
					Inline: true,
				},
			},
		}

		msg := discordgo.MessageSend{}
		if ind == 0 {
			embed.Description = tweet.Content
		}

		if media.Animated {
			resp, err := http.Get(media.URL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			filename := media.URL[strings.LastIndex(media.URL, "/")+1:]
			msg.File = &discordgo.File{
				Name:   filename,
				Reader: resp.Body,
			}
		} else {
			embed.Image = &discordgo.MessageEmbedImage{
				URL: media.URL,
			}
		}
		msg.Embed = &embed

		messages = append(messages, msg)
	}

	for _, message := range messages {
		_, err := s.ChannelMessageSendComplex(m.ChannelID, &message)
		if err != nil {
			return err
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
