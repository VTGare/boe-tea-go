package commands

import (
	"errors"
	"fmt"
	"image"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/internal/images"
	"github.com/VTGare/boe-tea-go/internal/repost"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/VTGare/sengoku"
	"github.com/bwmarrin/discordgo"
	log "github.com/sirupsen/logrus"
)

var (
	//ImageURLRegex is a regex for image URLs
	ImageURLRegex    = regexp.MustCompile(`(?i)(http(s?):)([/|.|\w|\s|-])*\.(?:jpg|jpeg|gif|png|webp)`)
	messageLinkRegex = regexp.MustCompile(`(?i)http(?:s)?:\/\/(?:www\.)?discord(?:app)?.com\/channels\/\d+\/(\d+)\/(\d+)`)
	sc               = sengoku.NewSengoku(os.Getenv("SAUCENAO_API"), sengoku.Config{
		DB:      sengoku.All,
		Results: 5,
	})
)

func init() {
	ig := Router.AddGroup(&gumi.Group{
		Name:        "images",
		Description: "Art posting and image manipulation commands",
		IsVisible:   true,
	})

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
		Description: "Embeds a tweet. Useful for mobile users.",
		Aliases:     []string{},
		Exec:        twitter,
		Cooldown:    5 * time.Second,
	})
	tCmd.Help = gumi.NewHelpSettings()
	tCmd.Help.AddField("Usage", "bt!twitter <tweet link> [excluded images]", false)
	tCmd.Help.AddField("Tweet link", "Required. Any tweet link is supported.", false)
	tCmd.Help.AddField("Excluded images", "Optional. Array of integer numbers. Ranges are supported (e.g. 1-3).", false)

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
	case strings.Contains(uri, "yande.re"):
		return fmt.Sprintf("[Yande.re](%v)", uri)
	default:
		return uri
	}
}

func joinSauceURLs(urls []string, sep string) string {
	var sb strings.Builder
	if len(urls) == 0 {
		return ""
	}

	sb.WriteString(namedLink(urls[0]))
	for _, uri := range urls[1:] {
		sb.WriteString(sep)
		sb.WriteString(namedLink(uri))
	}

	return sb.String()
}

func exclude(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	var (
		url     = args[0]
		indexes = args[1:]
	)

	art := repost.NewPost(m, url)
	if len(art.PixivMatches) == 0 {
		eb := embeds.NewBuilder()
		msg := "First argument should be a Pixiv link.\nValue received: [" + args[0] + "]"
		s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())
		return nil
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

	opts := repost.SendPixivOptions{
		IndexMap: indexMap,
	}
	err := art.Post(s, opts)
	if err != nil {
		return err
	}

	if user := database.DB.FindUser(m.Author.ID); user != nil {
		channels := user.Channels(m.ChannelID)
		err := art.Crosspost(s, channels, opts)
		if err != nil {
			return err
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
		indices = args[1:]
	)

	art := repost.NewPost(m, url)
	if len(art.PixivMatches) == 0 {
		eb := embeds.NewBuilder()
		msg := "First argument should be a Pixiv link.\nValue received: [" + args[0] + "]"
		s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())
		return nil
	}

	indexMap := make(map[int]bool)
	for _, ind := range indices {
		if strings.Contains(ind, "-") {
			ran, err := utils.NewRange(ind)
			if err != nil {
				return err
			}

			for i := ran.Low; i <= ran.High; i++ {
				indexMap[i] = true
			}
		} else {
			num, err := strconv.Atoi(ind)
			if err != nil {
				return utils.ErrParsingArgument
			}
			indexMap[num] = true
		}
	}

	opts := repost.SendPixivOptions{
		IndexMap: indexMap,
		Include:  true,
	}
	err := art.Post(s, opts)
	if err != nil {
		return err
	}

	if user := database.DB.FindUser(m.Author.ID); user != nil {
		channels := user.Channels(m.ChannelID)
		err := art.Crosspost(s, channels, opts)
		if err != nil {
			return err
		}
	}

	return nil
}

func crosspost(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("bt!crosspost requires at least one argument. **Usage:** bt!crosspost <pixiv link> [channel IDs]")
	}

	var (
		user = database.DB.FindUser(m.Author.ID)
		art  = repost.NewPost(m)
	)
	if user == nil {
		eb := embeds.NewBuilder()
		msg := "You have no cross-post groups. Please create one using a following command: ``bt!create <group name> <parent id>``"
		s.ChannelMessageSendEmbed(m.ChannelID, eb.FailureTemplate(msg).Finalize())
		return nil
	}

	err := art.Post(s)
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
		err := art.Crosspost(s, channels)
		if err != nil {
			return err
		}
	}

	return nil
}

func twitter(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	var (
		guild      = database.GuildCache[m.GuildID]
		twitterURL = args[0]
		indices    = args[1:]
	)

	indexMap := make(map[int]bool)
	for _, ind := range indices {
		if strings.Contains(ind, "-") {
			ran, err := utils.NewRange(ind)
			if err != nil {
				return err
			}

			for i := ran.Low; i <= ran.High; i++ {
				indexMap[i] = true
			}
		} else {
			num, err := strconv.Atoi(ind)
			if err != nil {
				return utils.ErrParsingArgument
			}
			indexMap[num] = true
		}
	}

	for index := range indexMap {
		if index < 1 || index > 4 {
			delete(indexMap, index)
		}
	}

	a := repost.NewPost(m, twitterURL)
	if guild.Repost != "disabled" {
		reposts := a.FindReposts(m.GuildID, m.ChannelID)
		if len(reposts) > 0 {
			switch guild.Repost {
			case "strict":
				s.ChannelMessageSendEmbed(m.ChannelID, a.RepostEmbed(reposts))
				return nil
			case "enabled":
				f := utils.CreatePromptWithMessage(s, m, &discordgo.MessageSend{
					Content: "Tweet you're trying to post is a repost. Are you sure about that?",
					Embed:   a.RepostEmbed(reposts),
				})
				if !f {
					return nil
				}
			}
		}
	}

	tweets, err := a.SendTwitter(s, a.TwitterMatches, false)
	if err != nil {
		return err
	}

	for _, t := range tweets {
		for ind, send := range t {
			if _, ok := indexMap[ind+1]; ok {
				continue
			}

			msg, err := s.ChannelMessageSendComplex(m.ChannelID, send)
			if err != nil {
				log.Warnln(err)
			}

			if msg != nil && guild.Reactions {
				s.MessageReactionAdd(msg.ChannelID, msg.ID, "💖")
				s.MessageReactionAdd(msg.ChannelID, msg.ID, "🤤")
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
