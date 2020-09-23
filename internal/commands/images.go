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
	"time"

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
	ImageURLRegex    = regexp.MustCompile(`(?i)(http(s?):)([/|.|\w|\s|-])*\.(?:jpg|jpeg|gif|png|webp)`)
	messageLinkRegex = regexp.MustCompile(`(?i)http(?:s)?:\/\/(?:www\.)?discord(?:app)?.com\/channels\/\d+\/(\d+)\/(\d+)`)
	sei              = seieki.NewSeieki(os.Getenv("SAUCENAO_API"))

	noSauceEmbed = &discordgo.MessageEmbed{
		Title:       "❎ Source material couldn't be found",
		Description: "Unfortunately Boe Tea couldn't find source of the provided image.\n\nOther reverse search engines are WIP, for now please consider using one of the following websites manually.",
		Fields: []*discordgo.MessageEmbedField{
			{"iqDB", "[Click here desu~](https://iqdb.org)", true},
			{"ASCII2D", "[Click here desu~](https://ascii2d.net)", true},
			{"SauceNAO", "[Click here desu~](https://saucenao.com)", true}},
		Timestamp: utils.EmbedTimestamp(),
		Thumbnail: &discordgo.MessageEmbedThumbnail{URL: utils.DefaultEmbedImage},
		Color:     utils.EmbedColor,
	}
)

func init() {
	ig := Router.AddGroup(&gumi.Group{
		Name:        "images",
		Description: "Source, repost and image manipulation commands",
		IsVisible:   true,
	})

	sauceCmd := ig.AddCommand(&gumi.Command{
		Name:        "sauce",
		Description: "Finds source of an anime picture using SauceNAO",
		Aliases:     []string{"saucenao", "snao"},
		Exec:        saucenao,
		Cooldown:    5 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})
	sauceCmd.Help.ExtendedHelp = []*discordgo.MessageEmbedField{
		{
			Name:  "Usage",
			Value: "bt!sauce <image link>",
		},
		{
			Name:  "image link",
			Value: "Required. Link must have one of the following suffixes:  *jpg*, *jpeg*, *png*, *gif*, *webp*.\nURL parameters (e.g. <link>.jpg***?width=441&height=441***) are fine too.",
		},
	}

	waitCmd := ig.AddCommand(&gumi.Command{
		Name:        "wait",
		Description: "Finds an anime source from a screenshot.",
		Aliases:     []string{"trace", "tracemoe"},
		Exec:        saucenao,
		Cooldown:    10 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})
	waitCmd.Help.ExtendedHelp = []*discordgo.MessageEmbedField{
		{
			Name:  "Usage",
			Value: "bt!wait <image link>",
		},
		{
			Name:  "image link",
			Value: "Required. Link must have one of the following suffixes:  *jpg*, *jpeg*, *png*, *gif*, *webp*.\nURL parameters (e.g. <link>.jpg***?width=441&height=441***) are fine too.",
		},
	}

	excludeCmd := ig.AddCommand(&gumi.Command{
		Name:        "exclude",
		Description: "Exclude selected images from a Pixiv album.",
		Aliases:     []string{"excl", "pixiv"},
		Exec:        exclude,
		Cooldown:    5 * time.Second,
	})
	excludeCmd.Help = gumi.NewHelpSettings().AddField("Usage", "bt!exclude <post link> [optional excluded images]", false)
	excludeCmd.Help.AddField("excluded images", "Integer numbers separated by whitespace (e.g. 1 3 5). Supports ranges like this 1-10. Ranges are inclusive.", false)

	includeCmd := ig.AddCommand(&gumi.Command{
		Name:        "include",
		Description: "Include only selected images from a Pixiv album.",
		Aliases:     []string{"incl"},
		Exec:        include,
		Cooldown:    5 * time.Second,
	})
	includeCmd.Help = gumi.NewHelpSettings().AddField("Usage", "bt!exclude <post link> [optional excluded images]", false)
	excludeCmd.Help.AddField("included images", "Integer numbers separated by whitespace (e.g. 1 3 5). Supports ranges like this 1-10. Ranges are inclusive.", false)

	dfCmd := ig.AddCommand(&gumi.Command{
		Name:        "deepfry",
		Description: "Deepfries an image, itadakimasu!",
		Aliases:     []string{},
		Exec:        deepfry,
		Cooldown:    15 * time.Second,
	})
	dfCmd.Help = gumi.NewHelpSettings()
	dfCmd.Help.AddField("Usage", "bt!deepfry <optional times deepfried> <image link>", false)
	dfCmd.Help.AddField("times deepfried", "Repeats deepfrying process given amount of times, up to 5.", false)

	tCmd := ig.AddCommand(&gumi.Command{
		Name:        "twitter",
		Description: "Embeds a Twitter link. Useful for posts with multiple images for mobile users",
		Aliases:     []string{},
		Exec:        twitter,
		Cooldown:    5 * time.Second,
	})
	tCmd.Help = gumi.NewHelpSettings()
	tCmd.Help.AddField("Usage", "bt!twitter <twitter link>", false)
	tCmd.Help.AddField("Twitter link", "Must look something like this: https://twitter.com/mhy_shima/status/1258684420011069442", false)

	jpegCmd := ig.AddCommand(&gumi.Command{
		Name:        "jpeg",
		Description: "Gives a provided image soul. 🙏",
		Aliases:     []string{"soul", "jpegify"},
		Exec:        jpegify,
		Cooldown:    15 * time.Second,
	})
	jpegCmd.Help = gumi.NewHelpSettings()
	jpegCmd.Help.AddField("Usage", "bt!jpeg <image quality> <image url>", false).AddField("image quality", "Optional integer from 0 to 100", false).AddField("image url", "Optional if attachment is present. Attachment is prioritized.", false)

	crosspostCmd := ig.AddCommand(&gumi.Command{
		Name:        "crosspost",
		Description: "Excludes provided channels from cross-posting a Twitter or Pixiv post.",
		Aliases:     []string{},
		Exec:        crosspost,
		Cooldown:    5 * time.Second,
		Help:        gumi.NewHelpSettings(),
	})
	crosspostCmd.Help.AddField("Usage", "bt!crosspost <twitter or pixiv link> [excluded channels]", false).AddField("Excluded channels", "IDs or mentions of channels you'd like to exclude from crossposting. Omit argument or give ``all`` to skip crossposting", false)
}

func saucenao(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	url, err := findImage(s, m, args)
	if err != nil {
		return err
	}

	if url == "" {
		return utils.ErrNotEnoughArguments
	}

	log.Infof("Searching source on SauceNAO. Image URL: %s", url)
	embed, err := saucenaoEmbed(url)
	if err != nil {
		return err
	}

	_, err = s.ChannelMessageSendEmbed(m.ChannelID, embed)
	if err != nil {
		return err
	}

	return nil
}

func wait(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	url, err := findImage(s, m, args)
	if err != nil {
		return err
	}

	if url == "" {
		return utils.ErrNotEnoughArguments
	}

	log.Infof("Searching source on trace.moe. Image URL: %s", url)
	embed, err := waitEmbed(url)
	if err != nil {
		return err
	}

	_, err = s.ChannelMessageSendEmbed(m.ChannelID, embed)
	if err != nil {
		return err
	}

	return nil
}

func findImage(s *discordgo.Session, m *discordgo.MessageCreate, args []string) (string, error) {
	if len(args) > 0 {
		if ImageURLRegex.MatchString(args[0]) {
			return args[0], nil
		} else if url, err := findImageFromMessageLink(s, args[0]); err == nil && url != "" {
			return url, nil
		}
	}

	if len(m.Attachments) > 0 {
		url := m.Attachments[0].URL
		if ImageURLRegex.MatchString(url) {
			return url, nil
		}
	}

	if len(m.Embeds) > 0 {
		if m.Embeds[0].Image != nil {
			url := m.Embeds[0].Image.URL
			if ImageURLRegex.MatchString(url) {
				return url, nil
			}
		}
	}

	messages, err := s.ChannelMessages(m.ChannelID, 5, m.ID, "", "")
	if err != nil {
		return "", err
	}
	if recent := findRecentImage(messages); recent != "" {
		return recent, nil
	}

	return "", nil
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

func findImageFromMessageLink(s *discordgo.Session, arg string) (string, error) {
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

func saucenaoEmbed(link string) (*discordgo.MessageEmbed, error) {
	res, err := sei.Sauce(link)
	if err != nil && res == nil {
		return nil, err
	}

	res.FilterLowSimilarity(60.0)
	if len(res.Results) == 0 {
		return noSauceEmbed, nil
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

func waitEmbed(link string) (*discordgo.MessageEmbed, error) {
	res, err := chotto.SearchWait(link)
	if err != nil {
		return nil, err
	}

	if len(res.Documents) == 0 {
		return noSauceEmbed, nil
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

func exclude(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	var (
		url     = args[0]
		indexes = args[1:]
	)

	post := func(m *discordgo.MessageCreate, crosspost bool) error {
		rep := repost.NewPost(*m, crosspost, url)
		if rep.Len() == 0 {
			return errors.New("first arguments must be a pixiv link")
		}

		guild := database.GuildCache[m.GuildID]
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

		indexMap := make(map[int]bool)
		for _, arg := range indexes {
			if strings.Contains(arg, "-") {
				ran, err := utils.NewRange(arg)
				if err != nil {
					return err
				}

				for i := ran.Low; i <= ran.High; i++ {
					indexMap[i] = true
				}
			} else {
				num, err := strconv.Atoi(arg)
				if err != nil {
					return utils.ErrParsingArgument
				}
				indexMap[num] = true
			}
		}

		messages, err := rep.SendPixiv(s, repost.SendPixivOptions{
			IndexMap: indexMap,
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

	err := post(m, false)
	if err != nil {
		return err
	}

	if user := database.DB.FindUser(m.Author.ID); user != nil {
		channels := user.Channels(m.ChannelID)
		for _, ch := range channels {
			c, err := s.State.Channel(ch)
			if err != nil {
				log.Warnln(err)
				continue
			}
			m.ChannelID = ch
			m.GuildID = c.GuildID

			err = post(m, true)
			if err != nil {
				log.Warnln(err)
			}
		}
	}

	return nil
}

func include(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	var (
		url     = args[0]
		indexes = args[1:]
	)

	post := func(m *discordgo.MessageCreate, crosspost bool) error {
		rep := repost.NewPost(*m, crosspost, url)
		if rep.Len() == 0 {
			return errors.New("first argument must be a pixiv link")
		}

		guild := database.GuildCache[m.GuildID]
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

		indexMap := make(map[int]bool)
		for _, arg := range indexes {
			if strings.Contains(arg, "-") {
				ran, err := utils.NewRange(arg)
				if err != nil {
					return err
				}

				for i := ran.Low; i <= ran.High; i++ {
					indexMap[i] = true
				}
			} else {
				num, err := strconv.Atoi(arg)
				if err != nil {
					return utils.ErrParsingArgument
				}
				indexMap[num] = true
			}
		}

		messages, err := rep.SendPixiv(s, repost.SendPixivOptions{
			IndexMap: indexMap,
			Include:  true,
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

	err := post(m, false)
	if err != nil {
		return err
	}

	if user := database.DB.FindUser(m.Author.ID); user != nil {
		channels := user.Channels(m.ChannelID)
		for _, ch := range channels {
			c, err := s.State.Channel(ch)
			if err != nil {
				log.Warnln(err)
				continue
			}

			log.Infof("channel name: %v, channel id: %v", c.Name, c.ID)
			m.ChannelID = c.ID
			m.GuildID = c.GuildID

			err = post(m, true)
			if err != nil {
				log.Warnln(err)
			}
		}
	}

	return nil
}

func crosspost(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("bt!exclude requires at least one argument. **Usage:** bt!exclude <pixiv link> [channel IDs]")
	}

	guild := database.GuildCache[m.GuildID]
	user := database.DB.FindUser(m.Author.ID)
	if user == nil {
		return fmt.Errorf("You have no cross-post groups. Please create one using a following command: ``bt!create <group name> <parent id>``")
	}
	user.Channels(m.ChannelID)

	post := func(m *discordgo.MessageCreate, crosspost bool) error {
		art := repost.NewPost(*m, crosspost, args[0])
		if art.Len() == 0 {
			return errors.New("first arguments must be a pixiv or a twitter link")
		}

		if guild.Repost != "disabled" {
			art.FindReposts()
			if len(art.Reposts) > 0 {
				if guild.Repost == "strict" {
					art.RemoveReposts()
					if crosspost {
						log.Infoln("found a repost while crossposting")
					}

					if !crosspost {
						s.ChannelMessageSendEmbed(m.ChannelID, art.RepostEmbed())
						perm, err := utils.MemberHasPermission(s, m.GuildID, s.State.User.ID, 8|8192)
						if err != nil {
							return err
						}

						if !perm {
							s.ChannelMessageSend(m.ChannelID, "Please enable Manage Messages permission to remove reposts with strict mode on, otherwise strict mode is useless.")
						} else if art.Len() == 0 {
							s.ChannelMessageDelete(m.ChannelID, m.ID)
						}
					}
				} else if guild.Repost == "enabled" && !crosspost {
					if art.PixivReposts() > 0 && guild.Pixiv {
						prompt := utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
							Content: "Following posts are reposts, react 👌 to post them.",
							Embed:   art.RepostEmbed(),
						})
						if !prompt {
							return nil
						}
					} else {
						s.ChannelMessageSendEmbed(m.ChannelID, art.RepostEmbed())
					}
				}
			}
		}

		if guild.Pixiv && len(art.PixivMatches) > 0 {
			messages, err := art.SendPixiv(s)
			if err != nil {
				return err
			}

			embeds := make([]*discordgo.Message, 0)
			keys := make([]string, 0)
			keys = append(keys, m.Message.ID)

			for _, message := range messages {
				embed, _ := s.ChannelMessageSendComplex(m.ChannelID, message)

				if embed != nil {
					keys = append(keys, embed.ID)
					embeds = append(embeds, embed)
				}
			}

			if art.HasUgoira {
				art.Cleanup()
			}

			c := &utils.CachedMessage{m.Message, embeds}
			for _, key := range keys {
				utils.MessageCache.Set(key, c)
			}
		}

		if (guild.Twitter || crosspost) && len(art.TwitterMatches) > 0 {
			tweets, err := art.SendTwitter(s, !crosspost)
			if err != nil {
				return err
			}

			if len(tweets) > 0 {
				msg := ""
				if len(tweets) == 1 {
					msg = "Detected a tweet with more than one image, would you like to send embeds of other images for mobile users?"
				} else {
					msg = "Detected tweets with more than one image, would you like to send embeds of other images for mobile users?"
				}

				prompt := true
				if !crosspost {
					prompt = utils.CreatePrompt(s, m, &utils.PromptOptions{
						Actions: map[string]bool{
							"👌": true,
						},
						Message: msg,
						Timeout: 10 * time.Second,
					})
				}

				if prompt {
					var (
						embeds = make([]*discordgo.Message, 0)
						keys   = make([]string, 0)
					)
					keys = append(keys, m.Message.ID)

					for _, t := range tweets {
						for _, send := range t {
							embed, err := s.ChannelMessageSendComplex(m.ChannelID, send)
							if err != nil {
								log.Warnln(err)
							}

							if embed != nil {
								keys = append(keys, embed.ID)
								embeds = append(embeds, embed)
							}
						}
					}

					c := &utils.CachedMessage{m.Message, embeds}
					for _, key := range keys {
						utils.MessageCache.Set(key, c)
					}
				}
			}
		}

		return nil
	}

	err := post(m, false)
	if err != nil {
		return err
	}

	if len(args) >= 2 {
		if args[1] == "all" {
			return nil
		}

		channels := utils.Filter(user.Channels(m.ChannelID), func(str string) bool {
			for _, a := range args[1:] {
				a = strings.Trim(a, "<#>")
				if a == str {
					return false
				}
			}
			return true
		})

		for _, ch := range channels {
			c, err := s.State.Channel(ch)
			if err != nil {
				continue
			}

			m.ChannelID = c.ID
			m.GuildID = c.GuildID

			err = post(m, true)
			if err != nil {
				return err
			}
		}
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
