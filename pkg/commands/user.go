package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/commands/flags"
	"github.com/VTGare/boe-tea-go/pkg/messages"
	"github.com/VTGare/boe-tea-go/pkg/models/artworks"
	"github.com/VTGare/boe-tea-go/pkg/models/artworks/options"
	"github.com/VTGare/boe-tea-go/pkg/models/users"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/mongo"
)

func userGroup(b *bot.Bot) {
	group := "user"
	b.Router.RegisterCmd(&gumi.Command{
		Name:        "groups",
		Group:       group,
		Aliases:     []string{"ls", "list"},
		Description: "Shows all crosspost groups.",
		Usage:       "bt!groups",
		Example:     "bt!groups",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        groups(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "newgroup",
		Group:       group,
		Aliases:     []string{"addgroup", "create"},
		Description: "Creates a new crosspost group.",
		Usage:       "bt!newgroup <group name> <parent channel>",
		Example:     "bt!newgroup lewds #nsfw",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        newgroup(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "delgroup",
		Group:       group,
		Description: "Deletes a crosspost group.",
		Usage:       "bt!delgroup <group name>",
		Example:     "bt!delgroup schooldays",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        delgroup(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "push",
		Group:       group,
		Aliases:     []string{},
		Description: "Adds channels to a crosspost group.",
		Usage:       "bt!push <group name> [channel ids]",
		Example:     "bt!push myCoolGroup #coolchannel #coolerchannel",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        push(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "remove",
		Group:       group,
		Aliases:     []string{"pop"},
		Description: "Removes channels from a crosspost group",
		Usage:       "bt!remove <group name> [channel ids]",
		Example:     "bt!remove cuteAnimeGirls #nsfw-channel #cat-pics",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        remove(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "copygroup",
		Group:       group,
		Aliases:     []string{},
		Description: "Copies a crosspost group with a different parent channel",
		Usage:       "bt!copygroup <from> <to> <parent channel id>",
		Example:     "bt!copygroup sfw1 sfw2 #za-warudo",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        copygroup(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "favourites",
		Group:       group,
		Aliases:     []string{"favorites", "favs"},
		Description: "Shows your favourites. Use help command to learn more about filtering and sorting.",
		Usage:       "bt!favourites [flags]",
		Example:     "bt!favourites during:month sort:time order:asc",
		Flags: map[string]string{
			"sort":   "**Options:** `[time, favourites]`. **Default:** time. Changes sort type.",
			"order":  "**Options:** `[asc, desc]`. **Default:** desc. Changes order of sorted artworks.",
			"mode":   "**Options:** `[all, sfw, nsfw]`. **Default:** all in nsfw channels and DMs, sfw otherwise.",
			"during": "**Options:** `[day, week, month]`. **Default:** none. Filters artworks by time.",
		},
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        favourites(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "unfav",
		Group:       group,
		Aliases:     []string{"unfavourite", "unfavorite"},
		Description: "Remove a favourite by its ID or URL",
		Usage:       "bt!unfav <artwork ID or URL>",
		Example:     "bt!unfav 69",
		RateLimiter: gumi.NewRateLimiter(15 * time.Second),
		Exec:        unfav(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "userset",
		Aliases:     []string{"profile"},
		Group:       group,
		Description: "Changes user's settings.",
		Usage:       "bt!userset <setting name> <new setting>",
		Example:     "bt!userset dm false",
		Flags: map[string]string{
			"dm":        "**Options:** `[on, off]`. Switches most direct messages from the bot.",
			"crosspost": "**Options:** `[on, off]`. Switches crossposting in general.",
		},
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        userset(b),
	})
}

func groups(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		user, err := b.Users.FindOneOrCreate(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		locale := messages.UserGroupsEmbed(ctx.Event.Author.Username)
		eb := embeds.NewBuilder()

		eb.Title(locale.Title)
		eb.Description(locale.Description)

		for _, group := range user.Groups {
			eb.AddField(
				locale.Group+" "+group.Name,
				fmt.Sprintf(
					"**%v:** %v\n **%v:**\n%v",
					locale.Parent, fmt.Sprintf(
						"<#%v> | `%v`",
						group.Parent, group.Parent,
					),
					locale.Children, strings.Join(arrays.MapString(
						group.Children,
						func(s string) string {
							return fmt.Sprintf("<#%v> | `%v`", s, s)
						},
					), "\n"),
				),
			)
		}

		ctx.ReplyEmbed(eb.Finalize())
		return nil
	}
}

func newgroup(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() < 2 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		user, err := b.Users.FindOneOrCreate(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		name := ctx.Args.Get(0).Raw
		parent := strings.Trim(ctx.Args.Get(1).Raw, "<#>")
		if _, err := ctx.Session.Channel(parent); err != nil {
			return messages.ErrChannelNotFound(err, parent)
		}

		_, err = b.Users.InsertGroup(context.Background(), user.ID, &users.Group{
			Name:     name,
			Parent:   parent,
			Children: []string{},
		})

		if err != nil {
			switch {
			case errors.Is(err, mongo.ErrNoDocuments):
				return messages.ErrInsertGroup(name, parent)
			default:
				return err
			}
		}

		eb := embeds.NewBuilder()

		eb.SuccessTemplate(fmt.Sprintf(
			"Created a group `%v` with parent channel <#%v> | `%v`", name, parent, parent,
		))

		ctx.ReplyEmbed(eb.Finalize())
		return nil
	}
}

func delgroup(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() < 1 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		user, err := b.Users.FindOneOrCreate(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		name := ctx.Args.Get(0).Raw
		_, err = b.Users.DeleteGroup(context.Background(), user.ID, name)

		if err != nil {
			switch {
			case errors.Is(err, mongo.ErrNoDocuments):
				return messages.ErrDeleteGroup(name)
			default:
				return err
			}
		}

		eb := embeds.NewBuilder()

		eb.SuccessTemplate(fmt.Sprintf(
			"Removed a group named `%v`", name,
		))

		ctx.ReplyEmbed(eb.Finalize())
		return nil
	}
}

func push(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() < 2 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		user, err := b.Users.FindOneOrCreate(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		name := ctx.Args.Get(0).Raw
		ctx.Args.Remove(0)

		group, ok := user.FindGroupByName(name)
		if !ok {
			return messages.ErrUserPushFail(name)
		}

		inserted := make([]string, 0, ctx.Args.Len())
		for _, arg := range ctx.Args.Arguments {
			channelID := strings.Trim(arg.Raw, "<#>")

			ch, err := ctx.Session.Channel(channelID)
			if err != nil {
				return messages.ErrChannelNotFound(err, channelID)
			}

			if ch.Type != discordgo.ChannelTypeGuildText {
				continue
			}

			if group.Parent == channelID {
				continue
			}

			if arrays.AnyString(group.Children, channelID) {
				continue
			}

			_, err = b.Users.InsertToGroup(
				context.Background(),
				user.ID,
				name,
				channelID,
			)

			if err != nil {
				switch {
				case errors.Is(err, mongo.ErrNoDocuments):
					continue
				default:
					return err
				}
			}

			inserted = append(inserted, channelID)
		}

		if len(inserted) > 0 {
			eb := embeds.NewBuilder()
			eb.SuccessTemplate(messages.UserPushSuccess(name, inserted))
			ctx.ReplyEmbed(eb.Finalize())
		} else {
			return messages.ErrUserPushFail(name)
		}

		return nil
	}
}

func remove(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() < 2 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		user, err := b.Users.FindOneOrCreate(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		name := ctx.Args.Get(0).Raw
		ctx.Args.Remove(0)

		group, ok := user.FindGroupByName(name)
		if !ok {
			return messages.ErrUserRemoveFail(name)
		}

		removed := make([]string, 0, ctx.Args.Len())
		for _, arg := range ctx.Args.Arguments {
			channelID := strings.Trim(arg.Raw, "<#>")

			if !arrays.AnyString(group.Children, channelID) {
				continue
			}

			_, err = b.Users.DeleteFromGroup(
				context.Background(),
				user.ID,
				name,
				channelID,
			)

			if err != nil {
				switch {
				case errors.Is(err, mongo.ErrNoDocuments):
					continue
				default:
					return err
				}
			}

			removed = append(removed, channelID)
		}

		if len(removed) > 0 {
			eb := embeds.NewBuilder()
			eb.SuccessTemplate(messages.UserRemoveSuccess(name, removed))
			ctx.ReplyEmbed(eb.Finalize())
		} else {
			return messages.ErrUserRemoveFail(name)
		}

		return nil
	}
}

func copygroup(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() < 3 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		user, err := b.Users.FindOneOrCreate(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		src := ctx.Args.Get(0).Raw
		dest := ctx.Args.Get(1).Raw
		parent := strings.Trim(ctx.Args.Get(2).Raw, "<#>")
		if _, ok := user.FindGroup(parent); ok {
			return messages.ErrUserChannelAlreadyParent(parent)
		}

		for _, group := range user.Groups {
			if group.Name == src {
				newGroup := &users.Group{
					Name:   dest,
					Parent: parent,
					Children: arrays.FilterString(group.Children, func(s string) bool {
						return s != parent
					}),
				}

				_, err := b.Users.InsertGroup(context.Background(), user.ID, newGroup)
				if err != nil {
					return messages.ErrUserCopyGroupFail(src, dest)
				}

				eb := embeds.NewBuilder()
				eb.SuccessTemplate(
					messages.UserCopyGroupSuccess(src, dest, newGroup.Children),
				)

				return ctx.ReplyEmbed(eb.Finalize())
			}
		}

		return messages.ErrUserCopyGroupFail(src, dest)
	}
}

func favourites(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		user, err := b.Users.FindOneOrCreate(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		if len(user.Favourites) == 0 {
			return messages.ErrUserNoFavourites(ctx.Event.Author.ID)
		}

		var (
			limit  = int64(len(user.Favourites))
			order  = options.Descending
			sort   = options.ByTime
			args   = strings.Fields(ctx.Args.Raw)
			mode   = flags.SFWFavourites
			filter = &options.Filter{}
		)

		ch, err := ctx.Session.Channel(ctx.Event.ChannelID)
		if err != nil {
			return err
		}

		if ch.NSFW || ch.Type == discordgo.ChannelTypeDM {
			mode = flags.AllFavourites
		}

		flagsMap, err := flags.FromArgs(
			args,
			flags.FlagTypeOrder,
			flags.FlagTypeSort,
			flags.FlagTypeDuring,
			flags.FlagTypeMode,
		)

		if err != nil {
			return err
		}

		for key, val := range flagsMap {
			switch key {
			case flags.FlagTypeOrder:
				order = val.(options.Order)
			case flags.FlagTypeSort:
				sort = val.(options.Sort)
			case flags.FlagTypeDuring:
				filter.Time = val.(time.Duration)
			case flags.FlagTypeMode:
				mode = val.(flags.FavouritesMode)
			}
		}

		ids := make([]int, 0, len(user.Favourites))
		switch mode {
		case flags.AllFavourites:
			for _, fav := range user.Favourites {
				ids = append(ids, fav.ArtworkID)
			}
		case flags.NSFWFavourites:
			for _, fav := range user.Favourites {
				if fav.NSFW {
					ids = append(ids, fav.ArtworkID)
				}
			}
		case flags.SFWFavourites:
			for _, fav := range user.Favourites {
				if !fav.NSFW {
					ids = append(ids, fav.ArtworkID)
				}
			}
		}

		filter.IDs = ids

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

		artworkEmbeds := make([]*discordgo.MessageEmbed, 0, len(artworks))
		for ind, artwork := range artworks {
			artworkEmbeds = append(artworkEmbeds, artworkToEmbed(artwork, artwork.Images[0], ind, len(artworks)))
		}

		wg := dgoutils.NewWidget(ctx.Session, ctx.Event.Author.ID, artworkEmbeds)
		return wg.Start(ctx.Event.ChannelID)
	}
}

func userset(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		switch {
		case ctx.Args.Len() == 0:
			user, err := b.Users.FindOneOrCreate(context.Background(), ctx.Event.Author.ID)
			if err != nil {
				return err
			}

			locale := messages.UserProfileEmbed(ctx.Event.Author.Username)
			eb := embeds.NewBuilder()
			eb.Title(locale.Title)
			eb.Thumbnail(ctx.Event.Author.AvatarURL(""))

			eb.AddField(
				locale.Settings,
				fmt.Sprintf(
					"**%v:** %v | **%v:** %v",
					locale.Crosspost, messages.FormatBool(user.Crosspost),
					locale.DM, messages.FormatBool(user.DM),
				),
			)

			eb.AddField(
				locale.Stats,
				fmt.Sprintf(
					"**%v:** %v | **%v:** %v",
					locale.Groups, len(user.Groups),
					locale.Favourites, len(user.Favourites),
				),
			)

			ctx.ReplyEmbed(eb.Finalize())
			return nil
		case ctx.Args.Len() >= 2:
			user, err := b.Users.FindOneOrCreate(context.Background(), ctx.Event.GuildID)
			if err != nil {
				return err
			}

			var (
				settingName     = ctx.Args.Get(0)
				newSetting      = ctx.Args.Get(1)
				newSettingEmbed interface{}
				oldSettingEmbed interface{}
			)

			switch settingName.Raw {
			case "dm":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = user.DM
				newSettingEmbed = new
				user.DM = new
			case "crosspost":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = user.Crosspost
				newSettingEmbed = new
				user.Crosspost = new
			default:
				return messages.ErrUnknownUserSetting(settingName.Raw)
			}

			_, err = b.Users.ReplaceOne(context.Background(), user)
			if err != nil {
				return err
			}

			eb := embeds.NewBuilder()
			eb.InfoTemplate("Successfully changed user setting.")
			eb.AddField("Setting name", settingName.Raw, true)
			eb.AddField("Old setting", fmt.Sprintf("%v", oldSettingEmbed), true)
			eb.AddField("New setting", fmt.Sprintf("%v", newSettingEmbed), true)

			ctx.ReplyEmbed(eb.Finalize())
			return nil
		default:
			return messages.ErrIncorrectCmd(ctx.Command)
		}
	}
}

func unfav(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() == 0 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		user, err := b.Users.FindOneOrCreate(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		if len(user.Favourites) == 0 {
			return messages.ErrUserNoFavourites(ctx.Event.Author.ID)
		}

		var (
			id    int
			url   string
			query = ctx.Args.Get(0).Raw
		)

		//If ID is not an integer assign query to the URL.
		if id, err = strconv.Atoi(query); err != nil {
			url = query
		}

		var artwork *artworks.Artwork
		if url != "" {
			artwork, err = b.Artworks.FindOne(context.Background(), &options.FilterOne{
				URL: url,
			})

			if err != nil {
				return messages.ErrArtworkNotFound(query)
			}

			id = artwork.ID
		}

		fav, ok := user.FindFavourite(id)
		if !ok {
			return messages.ErrArtworkNotFound(strconv.Itoa(id))
		}

		if _, err := b.Users.DeleteFavourite(context.Background(), user.ID, fav); err != nil {
			return messages.ErrUserUnfavouriteFail(query, err)
		}

		eb := embeds.NewBuilder()
		locale := messages.FavouriteRemovedEmbed()

		eb.Title(
			locale.Title,
		).Description(
			locale.Description,
		)

		eb.AddField(
			"ID",
			strconv.Itoa(artwork.ID),
			true,
		).AddField(
			"URL",
			messages.ClickHere(artwork.URL),
			true,
		).AddField(
			"NSFW",
			strconv.FormatBool(fav.NSFW),
			true,
		)

		if len(artwork.Images) > 0 {
			eb.Thumbnail(artwork.Images[0])
		}

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func artworkToEmbed(artwork *artworks.Artwork, image string, ind, length int) *discordgo.MessageEmbed {
	var (
		title   = ""
		percent = (float64(artwork.NSFW) / float64(artwork.Favourites)) * 100.0
	)

	if length > 1 {
		if artwork.Title == "" {
			title = fmt.Sprintf("[%v/%v] %v", ind+1, length, artwork.Author)
		} else {
			title = fmt.Sprintf("[%v/%v] %v", ind+1, length, artwork.Title)
		}
	} else {
		if artwork.Title == "" {
			title = fmt.Sprintf("%v", artwork.Author)
		} else {
			title = fmt.Sprintf("%v", artwork.Title)
		}
	}

	eb := embeds.NewBuilder()
	eb.Title(title).URL(artwork.URL)
	if len(artwork.Images) > 0 {
		eb.Image(image)
	}
	eb.AddField("ID",
		strconv.Itoa(artwork.ID),
		true,
	).AddField(
		"Author",
		artwork.Author,
		true,
	).AddField(
		"Favourites",
		strconv.Itoa(artwork.Favourites),
		true,
	).AddField(
		"URL",
		messages.ClickHere(artwork.URL),
	).AddField(
		"Lewdmeter",
		fmt.Sprintf("%.2f%s", percent, "%"),
	).Timestamp(artwork.CreatedAt)

	return eb.Finalize()
}
