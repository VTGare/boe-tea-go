package commands

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/commands/flags"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/post"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/mongo"
)

func artworksGroup(b *bot.Bot) {
	group := "artworks"

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "artwork",
		Group:       group,
		Aliases:     []string{},
		Description: "Embeds Boe Tea's artwork by its ID or parent URL.",
		Usage:       "bt!artwork <id or url>",
		Example:     "bt!artwork 69 OR bt!artwork https://pixiv.net/en/artworks/1234567",
		GuildOnly:   false,
		NSFW:        false,
		AuthorOnly:  false,
		Permissions: 0,
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        artwork(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "leaderboard",
		Group:       group,
		Aliases:     []string{"lb", "top"},
		Description: "Sends a leaderboard of saved Boe Tea's artworks",
		Usage:       "bt!leaderboard [flags]",
		Example:     "bt!leaderboard limit:123 during:week",
		Flags: map[string]string{
			"limit":  "**Options:** `any integer number up to 100`. **Default:** 100. Limits the size of a leaderboard.",
			"during": "**Options:** `[day, week, month]`. **Default:** all time. Filters artworks by time.",
		},
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        leaderboard(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "search",
		Group:       group,
		Aliases:     []string{},
		Description: "Search artworks in Boe Tea's database.",
		Usage:       "bt!search <query> [flags]",
		Example:     "bt!search hews limit:10 sort:popularity",
		Flags: map[string]string{
			"sort":   "**Options:** `[time, popularity]`. **Default:** time. Changes sort type.",
			"order":  "**Options:** `[asc, desc]`. **Default:** desc. Changes order of sorted artworks.",
			"limit":  "**Options:** `any integer number up to 100`. **Default:** 100. Limits the size of a leaderboard.",
			"during": "**Options:** `[day, week, month]`. **Default:** all time. Filters artworks by time.",
		},
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        search(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "share",
		Group:       group,
		Aliases:     []string{"pixiv", "twitter", "include", "shareinclude", "si"},
		Description: "Shares an artwork from a URL, optionally includes some images.",
		Usage:       "bt!share <artwork url> [indices to include]",
		Example:     "bt!share https://pixiv.net/artworks/86341538 1-3 5",
		GuildOnly:   true,
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        share(b, post.SkipModeInclude),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "shareexclude",
		Group:       group,
		Aliases:     []string{"exclude", "ex"},
		Description: "Shares an artwork from a URL, optionally excludes some images.",
		Usage:       "bt!ex <artwork url> [indices to exclude]",
		Example:     "bt!ex https://pixiv.net/artworks/86341538 1",
		GuildOnly:   true,
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        share(b, post.SkipModeExclude),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "crosspost",
		Group:       group,
		Aliases:     []string{"cp"},
		Description: "Shares an artwork from a URL without crossposting.",
		Usage:       "bt!crosspost <artwork url> [excluded channels (by default all)]",
		Example:     "bt!crosspost https://pixiv.net/artworks/86341538 #seiso-channel",
		GuildOnly:   true,
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        crosspost(b),
	})
}

func artwork(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() == 0 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		arg := ctx.Args.Get(0).Raw
		id, url, ok := parseArtworkArgument(arg)
		if !ok {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		artwork, err := b.Store.Artwork(context.Background(), id, url)
		if err != nil {
			switch {
			case errors.Is(err, mongo.ErrNoDocuments):
				return messages.ErrArtworkNotFound(arg)
			default:
				return err
			}
		}

		embeds := make([]*discordgo.MessageEmbed, 0, len(artwork.Images))
		for _, image := range artwork.Images {
			embed := artworkToEmbed(artwork, image, 0, 1)

			embeds = append(embeds, embed)
		}

		widget := dgoutils.NewWidget(ctx.Session, ctx.Event.Author.ID, embeds)
		return widget.Start(ctx.Event.ChannelID)
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

func share(b *bot.Bot, s post.SkipMode) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() < 1 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		//Trim <> in case someone wraps the link in it.
		url := strings.Trim(ctx.Args.Get(0).Raw, "<>")
		ctx.Args.Remove(0)

		indices := make(map[int]struct{})
		for _, arg := range strings.Fields(ctx.Args.Raw) {
			index, err := strconv.Atoi(arg)
			if err != nil {
				ran, err := dgoutils.NewRange(arg)
				if err != nil {
					return messages.ErrSkipIndexSyntax(arg)
				}

				for _, index := range ran.Array() {
					indices[index] = struct{}{}
				}
			} else {
				indices[index] = struct{}{}
			}
		}

		p := post.New(b, ctx, url)
		if len(indices) > 0 {
			p.SetSkip(indices, s)
		}

		allSent := make([]*cache.MessageInfo, 0)
		sent, err := p.Send()
		if err != nil {
			return err
		}

		allSent = append(allSent, sent...)

		user, _ := b.Store.User(context.Background(), ctx.Event.Author.ID)
		if user != nil {
			if channels, group, ok := user.FindGroupByParent(ctx.Event.ChannelID); ok {
				sent, err := p.Crosspost(user.ID, group.Name, channels)
				if err != nil {
					return err
				}

				allSent = append(allSent, sent...)
			}
		}

		if len(allSent) > 0 {
			b.EmbedCache.Set(
				ctx.Event.Author.ID,
				ctx.Event.ChannelID,
				ctx.Event.ID,
				true,
				allSent...,
			)

			for _, msg := range allSent {
				b.EmbedCache.Set(
					ctx.Event.Author.ID,
					msg.ChannelID,
					msg.MessageID,
					false,
				)
			}
		}

		return nil
	}
}

func crosspost(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() < 1 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		// Trim <> in case someone wraps the link in it.
		url := strings.Trim(ctx.Args.Get(0).Raw, "<>")
		ctx.Args.Remove(0)

		p := post.New(b, ctx, url)

		allSent := make([]*cache.MessageInfo, 0)
		sent, err := p.Send()
		if err != nil {
			return err
		}

		allSent = append(allSent, sent...)

		user, _ := b.Store.User(context.Background(), ctx.Event.Author.ID)
		if user != nil {
			if channels, group, ok := user.FindGroupByParent(ctx.Event.ChannelID); ok {
				excludedChannels := make(map[string]struct{})
				for _, arg := range strings.Fields(ctx.Args.Raw) {
					id := strings.Trim(arg, "<#>")
					excludedChannels[id] = struct{}{}
				}

				filtered := arrays.Filter(channels, func(s string) bool {
					_, ok := excludedChannels[s]
					return !ok
				})

				//If channels were successfully excluded, crosspost to a trimmed down
				//collection of channels. Otherwise skip crossposting altogether.
				if len(group.Children) > len(filtered) {
					sent, err := p.Crosspost(user.ID, group.Name, filtered)
					if err != nil {
						return err
					}

					allSent = append(allSent, sent...)
				}
			}
		}

		if len(allSent) > 0 {
			b.EmbedCache.Set(
				ctx.Event.Author.ID,
				ctx.Event.ChannelID,
				ctx.Event.ID,
				true,
				allSent...,
			)

			for _, msg := range allSent {
				b.EmbedCache.Set(
					ctx.Event.Author.ID,
					msg.ChannelID,
					msg.MessageID,
					false,
				)
			}
		}

		return nil
	}
}

func leaderboard(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		var (
			limit  int64 = 100
			args         = strings.Fields(ctx.Args.Raw)
			filter       = store.ArtworkFilter{}
		)

		flagsMap, err := flags.FromArgs(
			args,
			flags.FlagTypeDuring,
			flags.FlagTypeLimit,
		)

		if err != nil {
			return err
		}

		for key, val := range flagsMap {
			switch key {
			case flags.FlagTypeDuring:
				filter.Time = val.(time.Duration)
			case flags.FlagTypeLimit:
				limit = val.(int64)
				if limit > 100 {
					return messages.ErrLimitTooHigh(limit)
				}
			}
		}

		opts := store.ArtworkSearchOptions{
			Limit: limit,
			Order: store.Descending,
			Sort:  store.ByPopularity,
		}

		artworks, err := b.Store.SearchArtworks(context.Background(), filter, opts)
		if err != nil {
			return err
		}

		artworkEmbeds := make([]*discordgo.MessageEmbed, 0, len(artworks))

		ch, err := ctx.Session.Channel(ctx.Event.ChannelID)
		if err != nil {
			return messages.ErrChannelNotFound(err, ctx.Event.ChannelID)
		}

		if !ch.NSFW {
			locale := messages.SearchWarningEmbed()
			eb := embeds.NewBuilder()
			embed := eb.Title(locale.Title).Description(locale.Description).Finalize()

			artworkEmbeds = append(artworkEmbeds, embed)
		}

		for ind, artwork := range artworks {
			artworkEmbeds = append(artworkEmbeds, artworkToEmbed(artwork, artwork.Images[0], ind, len(artworks)))
		}

		wg := dgoutils.NewWidget(ctx.Session, ctx.Event.Author.ID, artworkEmbeds)
		return wg.Start(ctx.Event.ChannelID)
	}
}

func search(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() < 1 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		//Remove $'s to sanitize the input
		query := strings.Replace(ctx.Args.Get(0).Raw, "$", "", -1)

		var (
			limit  int64 = 100
			order        = store.Descending
			sort         = store.ByTime
			args         = strings.Fields(ctx.Args.Raw)
			filter       = store.ArtworkFilter{
				Query: query,
			}
		)

		flagsMap, err := flags.FromArgs(
			args,
			flags.FlagTypeDuring,
			flags.FlagTypeLimit,
			flags.FlagTypeSort,
			flags.FlagTypeOrder,
		)

		if err != nil {
			return err
		}

		for key, val := range flagsMap {
			switch key {
			case flags.FlagTypeDuring:
				filter.Time = val.(time.Duration)
			case flags.FlagTypeLimit:
				limit = val.(int64)
				if limit > 100 {
					return messages.ErrLimitTooHigh(limit)
				}
			case flags.FlagTypeOrder:
				order = val.(store.Order)
			case flags.FlagTypeSort:
				sort = val.(store.ArtworkSort)
			}
		}

		opts := store.ArtworkSearchOptions{
			Limit: limit,
			Order: order,
			Sort:  sort,
		}

		artworks, err := b.Store.SearchArtworks(context.Background(), filter, opts)

		if err != nil {
			return err
		}

		if len(artworks) == 0 {
			return messages.ErrArtworkNotFound(query)
		}

		artworkEmbeds := make([]*discordgo.MessageEmbed, 0, len(artworks))

		ch, err := ctx.Session.Channel(ctx.Event.ChannelID)
		if err != nil {
			return messages.ErrChannelNotFound(err, ctx.Event.ChannelID)
		}

		if !ch.NSFW {
			locale := messages.SearchWarningEmbed()
			eb := embeds.NewBuilder()
			embed := eb.Title(locale.Title).Description(locale.Description).Finalize()

			artworkEmbeds = append(artworkEmbeds, embed)
		}

		for ind, artwork := range artworks {
			artworkEmbeds = append(artworkEmbeds, artworkToEmbed(artwork, artwork.Images[0], ind, len(artworks)))
		}

		wg := dgoutils.NewWidget(ctx.Session, ctx.Event.Author.ID, artworkEmbeds)
		return wg.Start(ctx.Event.ChannelID)
	}
}
