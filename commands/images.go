package commands

import (
	"errors"
	"fmt"
	"image"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/images"
	"github.com/VTGare/boe-tea-go/nhentai"
	"github.com/VTGare/boe-tea-go/pixiv"
	"github.com/VTGare/boe-tea-go/saucenao"
	"github.com/VTGare/boe-tea-go/services"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	//ImageURLRegex is a regex for image URLs
	ImageURLRegex = regexp.MustCompile(`(?i)(http(s?):)([/|.|\w|\s|-])*\.(?:jpg|jpeg|gif|png|webp)`)
	ErrNoSauce    = errors.New("source image has not been found")
	searchEngines = map[string]func(link string) (*discordgo.MessageEmbed, error){
		"saucenao": snao,
		"wait":     wait,
	}
)

func init() {
	Commands["sauce"] = Command{
		Name:            "sauce",
		Description:     "Looks for source of an anime picture.",
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
				Value: "Not required. ``saucenao`` or ``wait``. If omitted uses server's default option",
			},
			{
				Name:  "image link",
				Value: "Required. Link must have one of the following suffixes:  *jpg*, *jpeg*, *png*, *gif*, *webp*.\nURL parameters after the link are acceptable (e.g. <link>.jpg***?width=441&height=441***)",
			},
		},
	}
	Commands["pixiv"] = Command{
		Name:            "pixiv",
		Description:     "Advanced pixiv command that lets you exclude images from an album",
		GuildOnly:       false,
		Exec:            pixivCommand,
		Help:            true,
		AdvancedCommand: true,
		ExtendedHelp: []*discordgo.MessageEmbedField{
			{
				Name:  "Usage",
				Value: "bt!pixiv <post link> [optional excluded images]",
			},
			{
				Name:  "excluded images",
				Value: "Indexes must be separated by whitespace (e.g. 1 2 4 6 10 45)",
			},
		},
	}
	Commands["deepfry"] = Command{
		Name:            "deepfry",
		Description:     "Deepfries an image, cursed as hell",
		GuildOnly:       false,
		Exec:            deepfry,
		Help:            true,
		AdvancedCommand: true,
		ExtendedHelp: []*discordgo.MessageEmbedField{
			{
				Name:  "Usage",
				Value: "bt!deepfry <optional times deepfried> <image link>",
			},
			{
				Name:  "times deepfried",
				Value: "Repeats deepfrying process given amount of times, up to 5.",
			},
			{
				Name:  "image link",
				Value: "Image link, if not present uses an attachment.",
			},
		},
	}
	Commands["twitter"] = Command{
		Name:            "twitter",
		Description:     "Reposts a twitter post",
		GuildOnly:       false,
		Exec:            twitter,
		Help:            true,
		AdvancedCommand: true,
		ExtendedHelp: []*discordgo.MessageEmbedField{
			{
				Name:  "Usage",
				Value: "bt!twitter <twitter link>",
			},
			{
				Name:  "Twitter link",
				Value: "Must look something like this: https://twitter.com/mhy_shima/status/1258684420011069442",
			},
		},
	}
	Commands["nhentai"] = Command{
		Name:            "nhentai",
		Description:     "Gives detailed information about an nhentai book",
		GuildOnly:       false,
		Exec:            nhen,
		Help:            true,
		AdvancedCommand: true,
		ExtendedHelp: []*discordgo.MessageEmbedField{
			{
				Name:  "Usage",
				Value: "bt!nhentai <magic number>",
			},
			{
				Name:  "magic number",
				Value: "Typically, but not always, 6-digit number that only weebs understand.",
			},
		},
	}
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

	url := ""
	searchEngine := ""

	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	} else if len(args) == 1 {
		searchEngine = findEngine()
		url = ImageURLRegex.FindString(args[0])
		if url == "" {
			return errors.New("received a non-image url")
		}
	} else if len(args) >= 2 {
		if f := ImageURLRegex.FindString(args[0]); f != "" {
			searchEngine = findEngine()
			url = f
		} else if _, ok := searchEngines[args[0]]; ok {
			searchEngine = args[0]
			url = ImageURLRegex.FindString(args[1])
		} else {
			return errors.New("incorrect command usage, please use bt!help sauce for more info")
		}

		if url == "" {
			return errors.New("received a non-image url")
		}
	}

	log.Infoln("Searching sauce for", url, "on", searchEngine)
	embed, err := searchEngines[searchEngine](url)
	if err != nil {
		return err
	}
	_, err = s.ChannelMessageSendEmbed(m.ChannelID, embed)
	if err != nil {
		return err
	}

	return nil
}

func snao(link string) (*discordgo.MessageEmbed, error) {
	res, err := saucenao.SearchSauceByURL(link)
	if err != nil {
		return nil, err
	}
	results, err := utils.FilterLowSimilarity(res.Results)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, ErrNoSauce
	}

	findSauce := func() *saucenao.Sauce {
		for _, res := range results {
			if len(res.Data.URLs) == 0 {
				continue
			}

			if res.Data.Title == "" {
				res.Data.Title = "Sauce"
			}

			return res
		}
		return nil
	}

	snaoSauce := findSauce()
	if res == nil {
		return nil, ErrNoSauce
	}

	author := utils.FindAuthor(*snaoSauce)
	log.Infoln("Source found. URL: %v. Title: %v", snaoSauce.Data.URLs[0], snaoSauce.Data.Title)
	embed := &discordgo.MessageEmbed{
		Title:     snaoSauce.Data.Title,
		URL:       snaoSauce.Data.URLs[0],
		Timestamp: utils.EmbedTimestamp(),
		Color:     utils.EmbedColor,
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: snaoSauce.Header.Thumbnail,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:  "URL",
				Value: snaoSauce.Data.URLs[0],
			},
			{
				Name:  "Similarity",
				Value: snaoSauce.Header.Similarity,
			},
			{
				Name:  "Author",
				Value: author,
			},
		},
	}
	return embed, nil
}

func wait(link string) (*discordgo.MessageEmbed, error) {
	res, err := services.SearchWait(link)
	if err != nil {
		return nil, err
	}

	if len(res.Documents) == 0 {
		return nil, errors.New("couldn't find source anime")
	}

	anime := res.Documents[0]

	description := ""
	url := ""
	if anime.AnilistID != 0 && anime.MalID != 0 {
		description = fmt.Sprintf("[AniList link](https://anilist.co/anime/%v/) | [MyAnimeList link](https://myanimelist.net/anime/%v/)", anime.AnilistID, anime.MalID)
		url = fmt.Sprintf("https://myanimelist.net/anime/%v/", anime.MalID)
	} else if anime.AnilistID != 0 {
		description = fmt.Sprintf("[AniList link](https://anilist.co/anime/%v/)", anime.AnilistID)
		url = fmt.Sprintf("https://anilist.co/anime/%v/", anime.AnilistID)
	} else if anime.MalID != 0 {
		description = fmt.Sprintf("[MyAnimeList link](https://myanimelist.net/anime/%v/)", anime.MalID)
		url = fmt.Sprintf("https://myanimelist.net/anime/%v/", anime.MalID)
	} else {
		description = "No links :shrug:"
	}

	embed := &discordgo.MessageEmbed{
		Title:       fmt.Sprintf("%v | %v", anime.TitleEnglish, anime.TitleNative),
		URL:         url,
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

func pixivCommand(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	match := pixiv.Regex.FindStringSubmatch(args[0])
	if match == nil {
		return errors.New("first arguments must be a pixiv link")
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

	pixiv.PostPixiv(s, m, []string{match[1]}, pixiv.Options{
		ProcPrompt: false,
		Indexes:    excludes,
		Exclude:    true,
	})
	return nil
}

func twitter(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	tweet, err := services.GetTweet(args[0])
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

	url := ""
	times := 0
	switch len(args) {
	case 2:
		if f := ImageURLRegex.FindString(args[0]); f != "" {
			url = f
		} else {
			var err error
			times, err = strconv.Atoi(args[0])
			if times > 5 {
				return errors.New("can't deepfry more than 5 times at once")
			}
			if err != nil {
				return err
			}
			url = ImageURLRegex.FindString(args[1])
		}

		if url == "" {
			return errors.New("received a non-image url")
		}
	case 1:
		if f := ImageURLRegex.FindString(args[0]); f != "" {
			url = f
		} else {
			return errors.New("received a non-image url")
		}
	case 0:
		return utils.ErrNotEnoughArguments
	}

	img, err := images.DownloadImage(url)
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

func nhen(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
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
	book, err := nhentai.GetNHentai(args[0])
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
