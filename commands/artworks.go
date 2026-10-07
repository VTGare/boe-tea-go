package commands

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/post"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/gumi/v2"
	"github.com/disgoorg/disgo/discord"
)

func artworksGroup(b *bot.Bot) []*gumi.Command {
	return []*gumi.Command{
		{
			Name:        "artwork",
			Category:    "Artworks",
			Description: "Embeds Boe Tea's artwork by its ID or parent URL.",
			Cooldown:    gumi.NewCooldown(gumi.CooldownUser, 1, 10*time.Second),
			Options: []*gumi.Option{
				gumi.String("query", "Artwork ID or URL").Require(),
			},
			Examples: []string{"artwork 69", "artwork https://pixiv.net/en/artworks/1234567"},
			Handler:  artwork(b),
		},
		{
			Name:        "leaderboard",
			Category:    "Artworks",
			Aliases:     []string{"lb", "top"},
			Description: "Sends a leaderboard of saved Boe Tea's artworks",
			Cooldown:    gumi.NewCooldown(gumi.CooldownUser, 1, 10*time.Second),
			Options: []*gumi.Option{
				gumi.Integer("limit", "Leaderboard size, up to 100"),
				gumi.String("during", "Only include recent artworks").WithChoices(
					gumi.Choice{Name: "day", Value: "day"},
					gumi.Choice{Name: "week", Value: "week"},
					gumi.Choice{Name: "month", Value: "month"},
				),
			},
			Examples: []string{"leaderboard 10 week"},
			Handler:  leaderboard(b),
		},
		{
			Name:        "search",
			Category:    "Artworks",
			Description: "Search artworks in Boe Tea's database.",
			Cooldown:    gumi.NewCooldown(gumi.CooldownUser, 1, 10*time.Second),
			Options: []*gumi.Option{
				gumi.String("query", "What to search for").Require(),
				gumi.Integer("limit", "Result size, up to 100"),
				gumi.String("sort", "How to sort results").WithChoices(
					gumi.Choice{Name: "time", Value: "time"},
					gumi.Choice{Name: "popularity", Value: "popularity"},
				),
				gumi.String("order", "Sort direction").WithChoices(
					gumi.Choice{Name: "asc", Value: "asc"},
					gumi.Choice{Name: "desc", Value: "desc"},
				),
				gumi.String("during", "Only include recent artworks").WithChoices(
					gumi.Choice{Name: "day", Value: "day"},
					gumi.Choice{Name: "week", Value: "week"},
					gumi.Choice{Name: "month", Value: "month"},
				),
			},
			Examples: []string{"search hews 10 popularity"},
			Handler:  search(b),
		},
		{
			Name:        "share",
			Category:    "Artworks",
			Aliases:     []string{"pixiv", "twitter", "include", "shareinclude", "si"},
			Description: "Shares an artwork from a URL, optionally picking which images to post.",
			Cooldown:    gumi.NewCooldown(gumi.CooldownUser, 1, 5*time.Second),
			Defer:       true,
			Options: []*gumi.Option{
				gumi.String("url", "Artwork URL").Require(),
				gumi.String("images", `Images to post (or remove if "mode" is set to "Exclude"), e.g. 1, 3-5`).Greedy(),
				gumi.String("mode", `Include or exclude listed images to post (set to "Include" by default)`).SlashOnly().WithChoices(
					gumi.Choice{Name: "Include", Value: "include"},
					gumi.Choice{Name: "Exclude", Value: "exclude"},
				),
				gumi.String("skip_channels", "Crosspost channels not to post this artwork to, e.g. #art #memes").SlashOnly(),
			},
			Examples: []string{"share https://pixiv.net/artworks/86341538 1-3 5"},
			Handler:  share(b, post.SkipModeInclude),
		},

		// Kept as prefix-only commands for legacy reasons.
		{
			Name:         "shareexclude",
			Category:     "Artworks",
			Aliases:      []string{"exclude", "ex"},
			Description:  "Shares an artwork from a URL, optionally excludes some images.",
			Cooldown:     gumi.NewCooldown(gumi.CooldownUser, 1, 5*time.Second),
			Defer:        true,
			DisableSlash: true,
			Options: []*gumi.Option{
				gumi.String("url", "Artwork URL").Require(),
				gumi.String("images", "Images to exclude, e.g. 1-3 5").Greedy(),
			},
			Examples: []string{"shareexclude https://pixiv.net/artworks/86341538 1"},
			Handler:  share(b, post.SkipModeExclude),
		},
		{
			Name:         "crosspostexclude",
			Category:     "Artworks",
			Aliases:      []string{"crosspost", "cp", "cpex"},
			Description:  "Shares an artwork from a URL without crossposting.",
			Checks:       []gumi.Check{gumi.GuildOnly},
			Cooldown:     gumi.NewCooldown(gumi.CooldownUser, 1, 5*time.Second),
			Defer:        true,
			DisableSlash: true,
			Options: []*gumi.Option{
				gumi.String("url", "Artwork URL").Require(),
				gumi.String("skip_channels", "Channels to exclude").Greedy(),
			},
			Examples: []string{"crosspostexclude https://pixiv.net/artworks/86341538 #seiso-channel"},
			Handler:  share(b, post.SkipModeInclude),
		},
		{
			Name:         "ignore",
			Category:     "Artworks",
			Description:  "Sends a message without reposting the artwork links in it.",
			DisableSlash: true,
			Options: []*gumi.Option{
				gumi.String("message", "Your message").Greedy(),
			},
			Examples: []string{"ignore https://pixiv.net/artworks/86341538 look at this"},
			Handler:  func(*gumi.Context) error { return nil },
		},
	}
}

func artwork(b *bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		arg := ctx.Options.String("query")
		id, url, ok := parseArtworkArgument(arg)
		if !ok {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 5*time.Second)
		defer cancel()

		var artwork *store.Artwork
		var err error
		if url != "" {
			artwork, err = b.FindArtwork(reqCtx, url)
		} else {
			artwork, err = b.Store.Artwork(reqCtx, id, "")
		}

		if err != nil {
			switch {
			case errors.Is(err, store.ErrArtworkNotFound):
				return messages.ErrArtworkNotFound(arg)
			default:
				return err
			}
		}

		if artwork == nil {
			return messages.ErrArtworkNotFound(arg)
		}

		embeds := make([]discord.Embed, 0, len(artwork.Images))
		for _, image := range artwork.Images {
			embed := artworkToEmbed(artwork, image, 0, 1)

			embeds = append(embeds, embed)
		}

		return replyPages(ctx, b, embeds)
	}
}

func parseArtworkArgument(arg string) (int, string, bool) {
	id, err := strconv.Atoi(arg)
	if err == nil {
		return id, "", true
	}

	_, err = url.ParseRequestURI(arg)
	if err == nil {
		return 0, arg, true
	}

	return 0, "", false
}

func parseSkipIndices(raw string) (map[int]struct{}, error) {
	indices := make(map[int]struct{})

	// Commas are allowed as separators too, e.g. "1, 3-5".
	for arg := range strings.FieldsSeq(strings.ReplaceAll(raw, ",", " ")) {
		index, err := strconv.Atoi(arg)
		if err != nil {
			ran, err := dgoutils.NewRange(arg)
			if err != nil {
				return nil, messages.ErrSkipIndexSyntax(arg)
			}

			for _, index := range ran.Array() {
				indices[index] = struct{}{}
			}
		} else {
			indices[index] = struct{}{}
		}
	}

	return indices, nil
}

// share posts an artwork. defaultMode is the prefix command's skip mode;
// the slash command picks one with the mode option instead.
func share(b *bot.Bot, defaultMode post.SkipMode) gumi.Handler {
	return func(ctx *gumi.Context) error {
		indices, err := parseSkipIndices(ctx.Options.String("images"))
		if err != nil {
			return err
		}

		mode := defaultMode
		if ctx.Options.String("mode") == "exclude" {
			mode = post.SkipModeExclude
		}

		p := post.NewPoster(post.DepsFromBot(b))
		run := post.RunFromMessage(runMessage(ctx), []string{dgoutils.TrimmerRaw(ctx.Options.String("url"))}, true)
		run.Skip = post.SkipFilter{Mode: mode, Indices: indices}
		run.IsInteraction = ctx.IsInteraction()

		for arg := range strings.FieldsSeq(ctx.Options.String("skip_channels")) {
			if id := dgoutils.ParseID(dgoutils.TrimmerRaw(arg)); id != 0 {
				run.ExcludedChannels = append(run.ExcludedChannels, id)
			}
		}

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 30*time.Second)
		defer cancel()

		sent, err := p.Send(reqCtx, run)
		post.CacheResult(b.EmbedCache, run.AuthorID, run.ChannelID, run.MessageID, sent)

		if err != nil {
			return err
		}

		// A prefix command's own message already shows who shared the link.
		// Only a slash command needs a reply, since Discord expects one.
		if !ctx.IsInteraction() {
			return nil
		}

		return shareAck(ctx, run)
	}
}

// shareAck says who shared which link. The link is wrapped in <> so
// Discord doesn't add a preview of its own.
func shareAck(ctx *gumi.Context, run post.Post) error {
	link := ""
	if len(run.URLs) > 0 {
		link = run.URLs[0]
	}

	eb := embeds.NewBuilder()
	eb.Description(fmt.Sprintf("%v shared <%v>", ctx.Author().Mention(), link))

	return ctx.Reply(gumi.Embed(eb.Finalize()))
}

func parseDuring(raw string) time.Duration {
	switch raw {
	case "day":
		return 24 * time.Hour
	case "week":
		return 7 * (24 * time.Hour)
	case "month":
		return 31 * (24 * time.Hour)
	default:
		return 0
	}
}

func warningEmbed(ch discord.Channel) []discord.Embed {
	if ch, ok := ch.(discord.GuildMessageChannel); ok && ch.NSFW() {
		return nil
	}

	locale := messages.SearchWarningEmbed()
	eb := embeds.NewBuilder()
	embed := eb.Title(locale.Title).Description(locale.Description).Finalize()

	return []discord.Embed{embed}
}

func leaderboard(b *bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		limit := ctx.Options.IntOr("limit", 100)
		if limit > 100 {
			return messages.ErrLimitTooHigh(limit)
		}

		filter := store.ArtworkFilter{Time: parseDuring(ctx.Options.String("during"))}
		opts := store.ArtworkSearchOptions{
			Limit: limit,
			Order: store.Descending,
			Sort:  store.ByPopularity,
		}

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 10*time.Second)
		defer cancel()

		found, err := b.Store.SearchArtworks(reqCtx, filter, opts)
		if err != nil {
			return err
		}

		ch, err := ctx.Channel()
		if err != nil {
			return messages.ErrChannelNotFound(err, ctx.ChannelID().String())
		}

		artworkEmbeds := warningEmbed(ch)
		for ind, artwork := range found {
			if artwork == nil {
				continue
			}

			artworkEmbeds = append(artworkEmbeds, artworkToEmbed(artwork, firstArtworkImage(artwork), ind, len(found)))
		}

		return replyPages(ctx, b, artworkEmbeds)
	}
}

func search(b *bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		// Remove $'s to sanitize the input
		query := strings.ReplaceAll(ctx.Options.String("query"), "$", "")

		limit := ctx.Options.IntOr("limit", 100)
		if limit > 100 {
			return messages.ErrLimitTooHigh(limit)
		}

		order := store.Descending
		if ctx.Options.String("order") == "asc" {
			order = store.Ascending
		}

		sortBy := store.ByTime
		if ctx.Options.String("sort") == "popularity" {
			sortBy = store.ByPopularity
		}

		filter := store.ArtworkFilter{
			Query: query,
			Time:  parseDuring(ctx.Options.String("during")),
		}
		opts := store.ArtworkSearchOptions{
			Limit: limit,
			Order: order,
			Sort:  sortBy,
		}

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 10*time.Second)
		defer cancel()

		found, err := b.Store.SearchArtworks(reqCtx, filter, opts)
		if err != nil {
			return err
		}

		if len(found) == 0 {
			return messages.ErrArtworkNotFound(query)
		}

		ch, err := ctx.Channel()
		if err != nil {
			return messages.ErrChannelNotFound(err, ctx.ChannelID().String())
		}

		artworkEmbeds := warningEmbed(ch)
		for ind, artwork := range found {
			if artwork == nil {
				continue
			}

			artworkEmbeds = append(artworkEmbeds, artworkToEmbed(artwork, firstArtworkImage(artwork), ind, len(found)))
		}

		return replyPages(ctx, b, artworkEmbeds)
	}
}
