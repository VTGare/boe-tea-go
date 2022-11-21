package commands

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"

	"github.com/VTGare/boe-tea-go/bot"
	nh "github.com/VTGare/boe-tea-go/internal/apis/nhentai"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
	"github.com/VTGare/sengoku"
	"github.com/bwmarrin/discordgo"
)

var (
	imageRegex      = regexp.MustCompile(`(?i)^https?://(?:[a-z0-9\-]+\.)+[a-z]{2,6}(?:/[^/#?]+)+\.(?:jpe?g|gif|png|webp)`)
	messageRefRegex = regexp.MustCompile(`(?i)http(?:s)?:\/\/(?:www\.)?discord(?:app)?.com\/channels\/\d+\/(\d+)\/(\d+)`)
)

func sourceGroup(b *bot.Bot) {
	group := "source"

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "sauce",
		Group:       group,
		Aliases:     []string{"saucenao"},
		Description: "Search sauce on SauceNAO",
		Example:     "bt!sauce https://imagehosting.com/animegirl.png",
		Usage:       "bt!sauce <image url, attachment, message url>",
		GuildOnly:   true,
		RateLimiter: gumi.NewRateLimiter(15 * time.Second),
		Exec:        sauce(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "nhentai",
		Group:       group,
		Aliases:     []string{"nh"},
		Description: "Displays more info about an nhentai doujin",
		Usage:       "bt!nhentai <nuke code>",
		Example:     "bt!nhentai 177013",
		NSFW:        true,
		RateLimiter: gumi.NewRateLimiter(15 * time.Second),
		Exec:        nhentai(b),
	})
}

func nhentai(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() == 0 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		id := ctx.Args.Get(0).Raw
		hentai, err := b.NHentai.FindHentai(id)
		if err != nil {
			switch {
			case errors.Is(err, nh.ErrNotFound):
				return messages.DoujinNotFound(id)
			case errors.Is(err, nh.ErrCloudflareProtection):
				return messages.CloudflareError()
			}

			return err
		}

		eb := embeds.NewBuilder()

		if hentai.Titles != nil {
			eb.Title(hentai.Titles.Pretty)
		} else {
			eb.Title("Unable to fetch the title")
		}

		eb.URL(hentai.URL)
		eb.Image(hentai.Cover)
		eb.Timestamp(hentai.UploadedAt)

		tagsToString := func(tags []*nh.Tag) string {
			ss := make([]string, 0, len(tags))
			for _, tag := range tags {
				ss = append(ss, tag.Name)
			}

			return strings.Join(ss, " • ")
		}

		tagsToNamedLinks := func(tags []*nh.Tag) string {
			ss := make([]string, 0, len(tags))
			for _, tag := range tags {
				ss = append(ss, messages.NamedLink(tag.Name, tag.URL))
			}

			return strings.Join(ss, " • ")
		}

		eb.AddField(
			"Pages", strconv.Itoa(hentai.Pages), true,
		).AddField(
			"Favourites", strconv.Itoa(hentai.Favourites), true,
		)

		if artists := hentai.Artists(); len(artists) > 0 {
			eb.AddField(
				"Artists",
				tagsToNamedLinks(artists),
				true,
			)
		}

		if characters := hentai.Characters(); len(characters) > 0 {
			eb.AddField(
				"Characters",
				tagsToNamedLinks(characters),
				true,
			)
		}

		if padories := hentai.Parodies(); len(padories) > 0 {
			eb.AddField(
				"Parodies",
				tagsToNamedLinks(padories),
				true,
			)
		}

		if lang, ok := hentai.Language(); ok {
			eb.AddField(
				"Language",
				cases.Title(language.English).String(lang.String()),
				true,
			)
		}

		if genres := hentai.Genres(); len(genres) > 0 {
			eb.AddField(
				"Tags",
				tagsToString(genres),
			)
		}

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func sauce(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		url, ok := findImage(
			ctx.Session,
			ctx.Event,
			strings.Fields(ctx.Args.Raw),
		)

		if !ok {
			return messages.SauceNoImage()
		}

		sauces, err := b.Sengoku.Search(url)
		if err != nil {
			switch {
			case errors.Is(err, sengoku.ErrRateLimitReached):
				return messages.SauceRateLimit()
			default:
				return messages.SauceError(err)
			}
		}

		filtered := make([]*sengoku.Sauce, 0)
		for _, sauce := range sauces {
			if sauce.Similarity > 70.0 && sauce.Pretty {
				filtered = append(filtered, sauce)
			}
		}

		if len(filtered) == 0 {
			return messages.SauceNotFound(url)
		}

		sauceEmbeds := sauceNAOEmbeds(filtered)
		widget := dgoutils.NewWidget(ctx.Session, ctx.Event.Author.ID, sauceEmbeds)
		return widget.Start(ctx.Event.ChannelID)
	}
}

func sauceNAOEmbeds(sauces []*sengoku.Sauce) []*discordgo.MessageEmbed {
	sauceEmbeds := make([]*discordgo.MessageEmbed, 0, len(sauces))
	locale := messages.SauceEmbed()

	toEmbed := func(sauce *sengoku.Sauce, index, l int) *discordgo.MessageEmbed {
		eb := embeds.NewBuilder()

		titleBuilder := strings.Builder{}
		if l > 1 {
			titleBuilder.WriteString(
				fmt.Sprintf("[%v/%v] ", index+1, l),
			)
		}

		if sauce.Title == "" {
			titleBuilder.WriteString(locale.NoTitle)
		} else {
			titleBuilder.WriteString(sauce.Title)
		}

		eb.Title(titleBuilder.String())
		if sauce.Author != nil {
			eb.AddField(
				locale.Author,
				messages.NamedLink(sauce.Author.Name, sauce.Author.URL),
			)
		}

		if sauce.URLs != nil {
			if uri, err := url.ParseRequestURI(sauce.URLs.Source); err == nil {
				eb.URL(uri.String())
				eb.AddField(
					"URL",
					uri.String(),
				)
			}

			if l := len(sauce.URLs.ExternalURLs); l != 0 {
				var sb strings.Builder
				uri := sauce.URLs.ExternalURLs[0]
				switch {
				case strings.Contains(uri, "twitter"):
					sb.WriteString(messages.NamedLink("Twitter", uri))
				case strings.Contains(uri, "danbooru"):
					sb.WriteString(messages.NamedLink("Danbooru", uri))
				case strings.Contains(uri, "gelbooru"):
					sb.WriteString(messages.NamedLink("Gelbooru", uri))
				default:
					sb.WriteString(messages.NamedLink(locale.ExternalURL+" 1", uri))
				}

				if l > 1 {
					for index, uri := range sauce.URLs.ExternalURLs[1:] {
						switch {
						case strings.Contains(uri, "twitter"):
							sb.WriteString(messages.NamedLink(" • Twitter", uri))
						case strings.Contains(uri, "danbooru"):
							sb.WriteString(messages.NamedLink(" • Danbooru", uri))
						case strings.Contains(uri, "gelbooru"):
							sb.WriteString(messages.NamedLink(" • Gelbooru", uri))
						default:
							sb.WriteString(messages.NamedLink(
								" • "+locale.ExternalURL+" "+strconv.Itoa(index+2),
								uri,
							))
						}
					}
				}

				eb.AddField(locale.OtherURLs, sb.String())
			}
		}

		eb.AddField(
			locale.Similarity,
			strconv.FormatFloat(sauce.Similarity, 'f', 2, 64),
		)

		eb.Thumbnail(sauce.Thumbnail)

		return eb.Finalize()
	}

	for index, sauce := range sauces {
		embed := toEmbed(sauce, index, len(sauces))
		sauceEmbeds = append(sauceEmbeds, embed)
	}

	return sauceEmbeds
}

func findImage(s *discordgo.Session, m *discordgo.MessageCreate, args []string) (string, bool) {
	if len(args) > 0 {
		if imageRegex.MatchString(args[0]) {
			return args[0], true
		} else if url, err := findImageMessageReference(s, args[0]); err == nil && url != "" {
			return url, true
		}
	}

	if len(m.Attachments) > 0 {
		url := m.Attachments[0].URL
		if imageRegex.MatchString(url) {
			return url, true
		}
	}

	if ref := m.MessageReference; ref != nil {
		url, err := findImageMessageReference(s, fmt.Sprintf("https://discord.com/channels/%s/%s/%s", ref.GuildID, ref.ChannelID, ref.MessageID))
		if err == nil && url != "" {
			return url, true
		}
	}

	if len(m.Embeds) > 0 {
		if m.Embeds[0].Image != nil {
			url := m.Embeds[0].Image.URL
			if imageRegex.MatchString(url) {
				return url, true
			}
		}
	}

	messages, err := s.ChannelMessages(m.ChannelID, 5, m.ID, "", "")
	if err != nil {
		return "", false
	}
	if recent := findImageMessages(messages); recent != "" {
		return recent, true
	}

	return "", false
}

func findImageMessages(messages []*discordgo.Message) string {
	for _, msg := range messages {
		f := imageRegex.FindString(msg.Content)
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

func findImageMessageReference(s *discordgo.Session, arg string) (string, error) {
	if matches := messageRefRegex.FindStringSubmatch(arg); matches != nil {
		m, err := s.ChannelMessage(matches[1], matches[2])
		if err != nil {
			return "", err
		}
		if recent := findImageMessages([]*discordgo.Message{m}); recent != "" {
			return recent, nil
		}
	}

	return "", nil
}
