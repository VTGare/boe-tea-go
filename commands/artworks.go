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
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/post"
	"github.com/VTGare/boe-tea-go/router"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"
	"github.com/bwmarrin/discordgo"
)

func artworksGroup(b *bot.Bot) []*router.Command {
	return []*router.Command{
		{
			Name:        "artwork",
			Category:    "Artworks",
			Description: "Embeds Boe Tea's artwork by its ID or parent URL.",
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 10*time.Second),
			Options: []*router.Option{
				router.String("query", "Artwork ID or URL").Require(),
			},
			Examples: []string{"artwork 69", "artwork https://pixiv.net/en/artworks/1234567"},
			Handler:  artwork(b),
		},
		{
			Name:        "leaderboard",
			Category:    "Artworks",
			Aliases:     []string{"lb", "top"},
			Description: "Sends a leaderboard of saved Boe Tea's artworks",
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 10*time.Second),
			Options: []*router.Option{
				router.Integer("limit", "Leaderboard size, up to 100"),
				router.String("during", "Only include recent artworks").WithChoices(
					router.Choice{Name: "day", Value: "day"},
					router.Choice{Name: "week", Value: "week"},
					router.Choice{Name: "month", Value: "month"},
				),
			},
			Examples: []string{"leaderboard 10 week"},
			Handler:  leaderboard(b),
		},
		{
			Name:        "search",
			Category:    "Artworks",
			Description: "Search artworks in Boe Tea's database.",
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 10*time.Second),
			Options: []*router.Option{
				router.String("query", "What to search for").Require(),
				router.Integer("limit", "Result size, up to 100"),
				router.String("sort", "How to sort results").WithChoices(
					router.Choice{Name: "time", Value: "time"},
					router.Choice{Name: "popularity", Value: "popularity"},
				),
				router.String("order", "Sort direction").WithChoices(
					router.Choice{Name: "asc", Value: "asc"},
					router.Choice{Name: "desc", Value: "desc"},
				),
				router.String("during", "Only include recent artworks").WithChoices(
					router.Choice{Name: "day", Value: "day"},
					router.Choice{Name: "week", Value: "week"},
					router.Choice{Name: "month", Value: "month"},
				),
			},
			Examples: []string{"search hews 10 popularity"},
			Handler:  search(b),
		},
		{
			Name:        "share",
			Category:    "Artworks",
			Aliases:     []string{"pixiv", "twitter", "include", "shareinclude", "si"},
			Description: "Shares an artwork from a URL, optionally includes some images.",
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 5*time.Second),
			Defer:       true,
			Options: []*router.Option{
				router.String("url", "Artwork URL").Require(),
				router.String("indices", "Images to include, e.g. 1-3 5").Greedy(),
			},
			Examples: []string{"share https://pixiv.net/artworks/86341538 1-3 5"},
			Handler:  share(b, post.SkipModeInclude),
		},
		{
			Name:        "shareexclude",
			Category:    "Artworks",
			Aliases:     []string{"exclude", "ex"},
			Description: "Shares an artwork from a URL, optionally excludes some images.",
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 5*time.Second),
			Defer:       true,
			Options: []*router.Option{
				router.String("url", "Artwork URL").Require(),
				router.String("indices", "Images to exclude, e.g. 1-3 5").Greedy(),
			},
			Examples: []string{"shareexclude https://pixiv.net/artworks/86341538 1"},
			Handler:  share(b, post.SkipModeExclude),
		},
		{
			Name:        "crosspostexclude",
			Category:    "Artworks",
			Aliases:     []string{"crosspost", "cp", "cpex"},
			Description: "Shares an artwork from a URL without crossposting.",
			Checks:      []router.Check{router.GuildOnly},
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 5*time.Second),
			Defer:       true,
			Options: []*router.Option{
				router.String("url", "Artwork URL").Require(),
				router.String("channels", "Channels to exclude").Greedy(),
			},
			Examples: []string{"crosspostexclude https://pixiv.net/artworks/86341538 #seiso-channel"},
			Handler:  crosspostExclude(b),
		},
	}
}

func artwork(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		arg := ctx.Options.String("query")
		id, url, ok := parseArtworkArgument(arg)
		if !ok {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 5*time.Second)
		defer cancel()

		artwork, err := b.Store.Artwork(reqCtx, id, url)
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

		embeds := make([]*discordgo.MessageEmbed, 0, len(artwork.Images))
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

	for _, arg := range strings.Fields(raw) {
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

func share(b *bot.Bot, skip post.SkipMode) router.Handler {
	return func(ctx *router.Context) error {
		indices, err := parseSkipIndices(ctx.Options.String("indices"))
		if err != nil {
			return err
		}

		p := post.NewPoster(post.DepsFromBot(b))
		run := post.RunFromMessage(runMessage(ctx), []string{dgoutils.TrimmerRaw(ctx.Options.String("url"))}, true)
		run.Skip = post.SkipFilter{Mode: skip, Indices: indices}
		run.IsInteraction = ctx.IsInteraction()

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 30*time.Second)
		defer cancel()

		sent, err := p.Send(reqCtx, run)
		post.CacheResult(b.EmbedCache, run.AuthorID, run.ChannelID, run.MessageID, sent)

		if err != nil {
			return err
		}

		return shareAck(ctx, run)
	}
}

func crosspostExclude(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		p := post.NewPoster(post.DepsFromBot(b))
		run := post.RunFromMessage(runMessage(ctx), []string{dgoutils.TrimmerRaw(ctx.Options.String("url"))}, true)
		run.IsInteraction = ctx.IsInteraction()

		for _, arg := range strings.Fields(ctx.Options.String("channels")) {
			run.ExcludedChannels = append(run.ExcludedChannels, dgoutils.TrimmerRaw(arg))
		}

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 30*time.Second)
		defer cancel()

		sent, err := p.Send(reqCtx, run)
		post.CacheResult(b.EmbedCache, run.AuthorID, run.ChannelID, run.MessageID, sent)

		if err != nil {
			return err
		}

		return shareAck(ctx, run)
	}
}

// shareAck replies with who shared which link. The link stays wrapped so
// Discord doesn't unfurl it into another embed.
func shareAck(ctx *router.Context, run post.Post) error {
	link := ""
	if len(run.URLs) > 0 {
		link = run.URLs[0]
	}

	eb := embeds.NewBuilder()
	eb.Description(fmt.Sprintf("%v shared <%v>", ctx.Author().Mention(), link))

	return ctx.Reply(router.Embed(eb.Finalize()))
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

func warningEmbed(ch *discordgo.Channel) []*discordgo.MessageEmbed {
	if ch == nil || ch.NSFW {
		return nil
	}

	locale := messages.SearchWarningEmbed()
	eb := embeds.NewBuilder()
	embed := eb.Title(locale.Title).Description(locale.Description).Finalize()

	return []*discordgo.MessageEmbed{embed}
}

func leaderboard(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
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

		ch, err := ctx.Session.Channel(ctx.ChannelID())
		if err != nil {
			return messages.ErrChannelNotFound(err, ctx.ChannelID())
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

func search(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
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

		ch, err := ctx.Session.Channel(ctx.ChannelID())
		if err != nil {
			return messages.ErrChannelNotFound(err, ctx.ChannelID())
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
