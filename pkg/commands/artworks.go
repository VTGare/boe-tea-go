package commands

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/cache"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/commands/flags"
	"github.com/VTGare/boe-tea-go/pkg/messages"
	"github.com/VTGare/boe-tea-go/pkg/models/artworks/options"
	"github.com/VTGare/boe-tea-go/pkg/post"
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
		Example:     "bt!search hews limit:10 sort:favourites",
		Flags: map[string]string{
			"sort":   "**Options:** `[time, favourites]`. **Default:** time. Changes sort type.",
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
		Aliases:     []string{"pixiv", "twitter", "exclude"},
		Description: "Shares an artwork from a URL, optionally excludes images.",
		Usage:       "bt!share <artwork url> [indices to exclude]",
		Example:     "bt!share https://pixiv.net/artworks/86341538 1-3 5",
		GuildOnly:   true,
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        share(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "shareinclude",
		Group:       group,
		Aliases:     []string{"si", "include"},
		Description: "Shares an artwork from a URL, optionally include only some images.",
		Usage:       "bt!si <artwork url> [indices to include]",
		Example:     "bt!si https://pixiv.net/artworks/86341538 1",
		GuildOnly:   true,
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        shareInclude(b),
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
		filter := argToArtworkFilter(arg)
		if filter == nil {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		artwork, err := b.Artworks.FindOne(context.Background(), filter)
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

func argToArtworkFilter(arg string) *options.FilterOne {
	id, err := strconv.Atoi(arg)
	if err == nil {
		return &options.FilterOne{
			ID: id,
		}
	}

	_, err = url.ParseRequestURI(arg)
	if err == nil {
		return &options.FilterOne{
			URL: arg,
		}
	}

	return nil
}

func share(b *bot.Bot) func(ctx *gumi.Ctx) error {
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
			p.SetSkip(indices, post.SkipModeExclude)
		}

		allSent := make([]*cache.MessageInfo, 0)
		sent, err := p.Send()
		if err != nil {
			return err
		}

		allSent = append(allSent, sent...)

		user, _ := b.Users.FindOne(context.Background(), ctx.Event.Author.ID)
		if user != nil {
			if group, ok := user.FindGroup(ctx.Event.ChannelID); ok {
				sent, err := p.Crosspost(user.ID, group.Name, group.Children)
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

func shareInclude(b *bot.Bot) func(ctx *gumi.Ctx) error {
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
			p.SetSkip(indices, post.SkipModeInclude)
		}

		allSent := make([]*cache.MessageInfo, 0)
		sent, err := p.Send()
		if err != nil {
			return err
		}

		allSent = append(allSent, sent...)

		user, _ := b.Users.FindOne(context.Background(), ctx.Event.Author.ID)
		if user != nil {
			if group, ok := user.FindGroup(ctx.Event.ChannelID); ok {
				sent, err := p.Crosspost(user.ID, group.Name, group.Children)
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

		//Trim <> in case someone wraps the link in it.
		url := strings.Trim(ctx.Args.Get(0).Raw, "<>")
		ctx.Args.Remove(0)

		p := post.New(b, ctx, url)

		allSent := make([]*cache.MessageInfo, 0)
		sent, err := p.Send()
		if err != nil {
			return err
		}

		allSent = append(allSent, sent...)

		user, _ := b.Users.FindOne(context.Background(), ctx.Event.Author.ID)
		if user != nil {
			if group, ok := user.FindGroup(ctx.Event.ChannelID); ok {
				excludedChannels := make(map[string]struct{})
				for _, arg := range strings.Fields(ctx.Args.Raw) {
					id := strings.Trim(arg, "<#>")
					excludedChannels[id] = struct{}{}
				}

				filtered := arrays.FilterString(group.Children, func(s string) bool {
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
			filter       = &options.Filter{}
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

		artworks, err := b.Artworks.FindMany(
			context.Background(),
			options.Find{
				Limit:  limit,
				Order:  options.Descending,
				Sort:   options.ByFavourites,
				Filter: filter,
			},
		)

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
			order        = options.Descending
			sort         = options.ByTime
			args         = strings.Fields(ctx.Args.Raw)
			filter       = &options.Filter{
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
				order = val.(options.Order)
			case flags.FlagTypeSort:
				sort = val.(options.Sort)
			}
		}

		artworks, err := b.Artworks.FindMany(
			context.Background(),
			options.Find{
				Limit:  limit,
				Order:  order,
				Sort:   sort,
				Filter: filter,
			},
		)

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
