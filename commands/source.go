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
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/gumi/v2"
	"github.com/VTGare/sengoku"
	disgobot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
	"github.com/julien040/go-ternary"
)

var (
	imageRegex      = regexp.MustCompile(`(?i)^https?://(?:[a-z0-9\-]+\.)+[a-z]{2,6}(?:/[^/#?]+)+\.(?:jpe?g|gif|png|webp)`)
	messageRefRegex = regexp.MustCompile(`(?i)http(?:s)?:\/\/(?:www\.)?discord(?:app)?.com\/channels\/\d+\/(\d+)\/(\d+)`)
	pximgRegex      = regexp.MustCompile(`(?i)https?://i\.pximg\.net/.+?/(\d+)(?:_p\d+)?(?:\.[a-z]+)?(?:$|[?#])`)
)

func sourceGroup(b *bot.Bot) []*gumi.Command {
	return []*gumi.Command{
		{
			Name:        "sauce",
			Category:    "Source",
			Aliases:     []string{"saucenao"},
			Description: "Search sauce on SauceNAO",
			Cooldown:    gumi.NewCooldown(gumi.CooldownUser, 1, 5*time.Second),
			Options: []*gumi.Option{
				gumi.String("url", "Image URL or Discord message link").Greedy(),
				gumi.Attachment("image", "Image to look up"),
				gumi.Boolean("private", "Only you can see the results").SlashOnly(),
			},
			Examples: []string{"sauce https://imagehosting.com/animegirl.png"},
			Handler:  sauce(b),
		},
		{
			Name:        "Find Sauce",
			Category:    "Source",
			Type:        gumi.MessageContext,
			Description: "Find the source of images in a message",
			Cooldown:    gumi.NewCooldown(gumi.CooldownUser, 1, 5*time.Second),
			Handler:     sauce(b),
		},
		{
			Name:          "Find Sauce (Private)",
			Category:      "Source",
			Type:          gumi.MessageContext,
			Description:   "Find the source of images in a message, only you see the results",
			Cooldown:      gumi.NewCooldown(gumi.CooldownUser, 1, 5*time.Second),
			Ephemeral:     true,
			DisablePrefix: true,
			Handler:       sauce(b),
		},
	}
}

func sauce(b *bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		if ctx.Options.Bool("private") {
			ctx.SetEphemeral(true)
		}

		if err := ctx.Defer(); err != nil {
			return err
		}

		if ctx.TargetMessage != nil {
			url, ok := findImage(ctx.Client, ctx.TargetMessage, nil)
			if !ok {
				return messages.SauceNoImage()
			}

			return searchSauce(ctx, b, url)
		}

		var (
			imageURL string
			url      string
			ok       bool
		)

		if att := ctx.Options.Attachment("image"); att != nil {
			imageURL = att.URL
		}

		query := ctx.Options.String("url")

		switch {
		case imageURL != "":
			url, ok = imageURL, true
		case ctx.IsMessage():
			url, ok = findImage(ctx.Client, ctx.Message, strings.Fields(query))
		case query != "":
			if imageRegex.MatchString(query) {
				url, ok = query, true
			} else if ref, err := findImageMessageReference(ctx.Client, query); err == nil && ref != "" {
				url, ok = ref, true
			}
		}

		if !ok {
			return messages.SauceNoImage()
		}

		return searchSauce(ctx, b, url)
	}
}

func searchSauce(ctx *gumi.Context, b *bot.Bot, url string) error {
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

	return replyPages(ctx, b, sauceNAOEmbeds(filtered))
}

func sauceNAOEmbeds(sauces []*sengoku.Sauce) []discord.Embed {
	sauceEmbeds := make([]discord.Embed, 0, len(sauces))

	toEmbed := func(source *sengoku.Sauce, index, l int) discord.Embed {
		eb := embeds.NewBuilder()

		titleBuilder := strings.Builder{}
		if l > 1 {
			fmt.Fprintf(&titleBuilder, "[%v/%v] ", index+1, l)
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

func findImage(c *disgobot.Client, m *discord.Message, args []string) (string, bool) {
	if len(args) > 0 {
		if imageRegex.MatchString(args[0]) {
			return args[0], true
		} else if url, err := findImageMessageReference(c, args[0]); err == nil && url != "" {
			return url, true
		}
	}

	if len(m.Attachments) > 0 {
		url := m.Attachments[0].URL
		if imageRegex.MatchString(url) {
			return url, true
		}
	}

	if ref := m.MessageReference; ref != nil && ref.ChannelID != nil && ref.MessageID != nil {
		if url, err := findImageInMessage(c, *ref.ChannelID, *ref.MessageID); err == nil && url != "" {
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

	messages, err := c.Rest.GetMessages(m.ChannelID, 0, m.ID, 0, 5)
	if err != nil {
		return "", false
	}
	if recent := findImageMessages(messages); recent != "" {
		return recent, true
	}

	return "", false
}

func findImageMessages(messages []discord.Message) string {
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

func findImageMessageReference(c *disgobot.Client, arg string) (string, error) {
	if matches := messageRefRegex.FindStringSubmatch(arg); matches != nil {
		return findImageInMessage(c, dgoutils.ParseID(matches[1]), dgoutils.ParseID(matches[2]))
	}

	return "", nil
}

func findImageInMessage(c *disgobot.Client, channelID, messageID snowflake.ID) (string, error) {
	m, err := c.Rest.GetMessage(channelID, messageID)
	if err != nil {
		return "", err
	}

	return findImageMessages([]discord.Message{*m}), nil
}
