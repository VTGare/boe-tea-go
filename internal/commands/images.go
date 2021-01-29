package commands

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/internal/repost"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/VTGare/sengoku"
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
	groupName := "images"
	Commands = append(Commands, &gumi.Command{
		Name:        "crosspost",
		Description: "Excludes given channels from crossposting.",
		Group:       groupName,
		Usage:       "bt!crosspost <artwork link> [optional channels]",
		Example:     "bt!crosspost https://pixiv.net/artworks/1337420 #sfw",
		Flags:       map[string]string{"Optional channels": "If no channels were provided excludes all channels instead."},
		Exec:        crosspost,
		GuildOnly:   true,
	})

	Commands = append(Commands, &gumi.Command{
		Name:        "twitter",
		Aliases:     []string{"tweet"},
		Description: "Embeds a tweet.",
		Group:       groupName,
		Usage:       "bt!twitter <tweet link> [optional excluded images]",
		Example:     "bt!twitter https://twitter.com/Zephyroh/status/1354471389496037378 1",
		Exec:        twitter,
	})

	Commands = append(Commands, &gumi.Command{
		Name:        "include",
		Aliases:     []string{"incl"},
		Group:       groupName,
		Description: "Includes only given images from a pixiv album",
		Usage:       "bt!include <pixiv artwork link> [included images]",
		Example:     "bt!include https://www.pixiv.net/en/artworks/87329172 1-3",
		Flags:       map[string]string{"included images": "Integer numbers separated by whitespace (e.g. 1 3 5). Supports ranges like this 1-10. Ranges are inclusive."},
		Exec:        include,
	})

	Commands = append(Commands, &gumi.Command{
		Name:        "exclude",
		Aliases:     []string{"excl", "pixiv"},
		Group:       groupName,
		Description: "Excludes given images from a pixiv album",
		Usage:       "bt!exclude <pixiv artwork link> [excluded images]",
		Example:     "bt!exclude https://www.pixiv.net/en/artworks/87329172 1-3",
		Flags:       map[string]string{"excluded images": "Integer numbers separated by whitespace (e.g. 1 3 5). Supports ranges like this 1-10. Ranges are inclusive."},
		Exec:        exclude,
	})
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

func exclude(ctx *gumi.Ctx) error {
	if ctx.Args.Len() == 0 {
		return utils.ErrNotEnoughArguments
	}

	args := strings.Fields(ctx.Args.Raw)
	var (
		s       = ctx.Session
		m       = ctx.Event
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
	for _, arg := range indices {
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

	opts := repost.RepostOptions{
		PixivIndices: indexMap,
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

func include(ctx *gumi.Ctx) error {
	if ctx.Args.Len() == 0 {
		return utils.ErrNotEnoughArguments
	}

	args := strings.Fields(ctx.Args.Raw)
	var (
		s       = ctx.Session
		m       = ctx.Event
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

	opts := repost.RepostOptions{
		PixivIndices: indexMap,
		Include:      true,
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

func crosspost(ctx *gumi.Ctx) error {
	if ctx.Args.Len() < 1 {
		return fmt.Errorf("bt!crosspost requires at least one argument. **Usage:** bt!crosspost <pixiv link> [channel IDs]")
	}

	var (
		s    = ctx.Session
		m    = ctx.Event
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
	if ctx.Args.Len() >= 2 {
		if ctx.Args.Get(1).Raw == "all" {
			return nil
		}

		channels := utils.Filter(user.Channels(m.ChannelID), func(str string) bool {
			for _, a := range ctx.Args.Arguments[1:] {
				a.Raw = strings.Trim(a.Raw, "<#>")
				if a.Raw == str {
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

func twitter(ctx *gumi.Ctx) error {
	if ctx.Args.Len() == 0 {
		return utils.ErrNotEnoughArguments
	}

	var (
		m          = ctx.Event
		s          = ctx.Session
		args       = strings.Fields(ctx.Args.Raw)
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
	err := a.Post(s, repost.RepostOptions{
		KeepTwitterFirst:  true,
		TwitterIndices:    indexMap,
		SkipTwitterPrompt: true,
	})

	return err
}
