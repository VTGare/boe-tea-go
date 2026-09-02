package commands

import (
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
	"github.com/VTGare/sengoku"
	"github.com/bwmarrin/discordgo"
	"github.com/julien040/go-ternary"
)

var (
	imageRegex      = regexp.MustCompile(`(?i)^https?://(?:[a-z0-9\-]+\.)+[a-z]{2,6}(?:/[^/#?]+)+\.(?:jpe?g|gif|png|webp)`)
	messageRefRegex = regexp.MustCompile(`(?i)http(?:s)?:\/\/(?:www\.)?discord(?:app)?.com\/channels\/\d+\/(\d+)\/(\d+)`)
	pximgRegex = regexp.MustCompile(`(?i)https?://i\.pximg\.net/.+?/(\d+)(?:_p\d+)?(?:\.[a-z]+)?(?:$|[?#])`)
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
}

func sauce(b *bot.Bot) func(*gumi.Ctx) error {
	return func(gctx *gumi.Ctx) error {
		url, ok := findImage(
			gctx.Session,
			gctx.Event,
			strings.Fields(gctx.Args.Raw),
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
		widget := dgoutils.NewWidget(gctx.Session, gctx.Event.Author.ID, sauceEmbeds)
		return widget.Start(gctx.Event.ChannelID)
	}
}

func sauceNAOEmbeds(sauces []*sengoku.Sauce) []*discordgo.MessageEmbed {
	sauceEmbeds := make([]*discordgo.MessageEmbed, 0, len(sauces))

	toEmbed := func(source *sengoku.Sauce, index, l int) *discordgo.MessageEmbed {
		eb := embeds.NewBuilder()

		titleBuilder := strings.Builder{}
		if l > 1 {
			titleBuilder.WriteString(fmt.Sprintf("[%v/%v] ", index+1, l))
		}

		titleBuilder.WriteString(ternary.If(
			source.Title == "",
			"No title",
			source.Title,
		))

		eb.Title(titleBuilder.String())
		if source.Author != nil {
			eb.AddField("Artist", messages.NamedLink(source.Author.Name, source.Author.URL))
		}

		if source.URLs != nil {
			handleURLs(source, eb)
		}

		eb.AddField("Similarity", strconv.FormatFloat(source.Similarity, 'f', 2, 64))
		eb.Thumbnail(source.Thumbnail)

		return eb.Finalize()
	}

	for index, sauce := range sauces {
		embed := toEmbed(sauce, index, len(sauces))
		sauceEmbeds = append(sauceEmbeds, embed)
	}

	return sauceEmbeds
}

func handleURLs(source *sengoku.Sauce, eb *embeds.Builder) {
	sourceURL := source.URLs.Source
	if pixivURL, ok := pixivArtworkURL(sourceURL); ok {
		sourceURL = pixivURL
	}

	if uri, err := url.ParseRequestURI(sourceURL); err == nil {
		eb.URL(uri.String())
		eb.AddField("URL", uri.String())
	}

	if len(source.URLs.ExternalURLs) == 0 {
		return
	}

	var sb strings.Builder
	uri := source.URLs.ExternalURLs[0]

	switch {
	case strings.Contains(uri, "twitter"):
		sb.WriteString(messages.NamedLink("Twitter", uri))
	case strings.Contains(uri, "danbooru"):
		sb.WriteString(messages.NamedLink("Danbooru", uri))
	case strings.Contains(uri, "gelbooru"):
		sb.WriteString(messages.NamedLink("Gelbooru", uri))
	default:
		sb.WriteString(messages.NamedLink("URL 1", uri))
	}

	if len(source.URLs.ExternalURLs) <= 1 {
		return
	}

	for index, uri := range source.URLs.ExternalURLs[1:] {
		switch {
		case strings.Contains(uri, "twitter"):
			sb.WriteString(messages.NamedLink(" • Twitter", uri))
		case strings.Contains(uri, "danbooru"):
			sb.WriteString(messages.NamedLink(" • Danbooru", uri))
		case strings.Contains(uri, "gelbooru"):
			sb.WriteString(messages.NamedLink(" • Gelbooru", uri))
		default:
			sb.WriteString(messages.NamedLink(" • URL"+" "+strconv.Itoa(index+2), uri))
		}
	}

	eb.AddField("External links", sb.String())
}

func pixivArtworkURL(raw string) (string, bool) {
	matches := pximgRegex.FindStringSubmatch(raw)
	if len(matches) < 2 {
		return raw, false
	}

	return fmt.Sprintf("https://pixiv.net/artworks/%s", matches[1]), true
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
