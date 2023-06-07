package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/commands/flags"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/mongo"
)

// userGroup registers user group commands.
func userGroup(b *bot.Bot) {
	// Assigns type user for command group.
	group := "user"

	// Registers groups command.
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

	// Registers newgroup command.
	b.Router.RegisterCmd(&gumi.Command{
		Name:        "newgroup",
		Group:       group,
		Aliases:     []string{"addgroup", "create"},
		Description: "Creates a new crosspost group.",
		Usage:       "bt!newgroup <group name> <parent channel>",
		Example:     "bt!newgroup lewds #nsfw",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        newgroup(b, false),
	})

	// Registers newpair command.
	b.Router.RegisterCmd(&gumi.Command{
		Name:        "newpair",
		Group:       group,
		Aliases:     []string{"addpair"},
		Description: "Creates a new crosspost pair.",
		Usage:       "bt!newpair <pair name> <first channel> <second channel>",
		Example:     "bt!newpair lewds #nsfw #nsfw-pics",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        newgroup(b, true),
	})

	// Registers delgroup command.
	b.Router.RegisterCmd(&gumi.Command{
		Name:        "delgroup",
		Group:       group,
		Description: "Deletes a crosspost group.",
		Usage:       "bt!delgroup <group name>",
		Example:     "bt!delgroup schooldays",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        delgroup(b),
	})

	// Registers push command.
	b.Router.RegisterCmd(&gumi.Command{
		Name:        "push",
		Group:       group,
		Description: "Adds channels to a crosspost group.",
		Usage:       "bt!push <group name> [channel ids]",
		Example:     "bt!push myCoolGroup #coolchannel #coolerchannel",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        push(b),
	})

	// Registers remove command.
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

	// Registers rename command.
	b.Router.RegisterCmd(&gumi.Command{
		Name:        "rename",
		Group:       group,
		Description: "Renames a crosspost group",
		Usage:       "bt!rename <from> <to>",
		Example:     "bt!rename cuteAnimeGirls AnimeGirls",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        rename(b),
	})

	// Registers copygroup command.
	b.Router.RegisterCmd(&gumi.Command{
		Name:        "copygroup",
		Group:       group,
		Description: "Copies a crosspost group with a different parent channel",
		Usage:       "bt!copygroup <from> <to> <parent channel id>",
		Example:     "bt!copygroup sfw1 sfw2 #za-warudo",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        copygroup(b),
	})

	// Registers bookmarks command.
	b.Router.RegisterCmd(&gumi.Command{
		Name:        "bookmarks",
		Group:       group,
		Aliases:     []string{"favorites", "favourites", "favs"},
		Description: "Shows your bookmarks. Use help command to learn more about filtering and sorting.",
		Usage:       "bt!bookmarks [flags]",
		Example:     "bt!bookmarks during:month sort:time order:asc",
		Flags: map[string]string{
			"sort":   "**Options:** `[time, popularity]`. **Default:** time. Changes sort type.",
			"order":  "**Options:** `[asc, desc]`. **Default:** desc. Changes order of sorted artworks.",
			"mode":   "**Options:** `[all, sfw, nsfw]`. **Default:** all in nsfw channels and DMs, sfw otherwise.",
			"during": "**Options:** `[day, week, month]`. **Default:** none. Filters artworks by time.",
		},
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        bookmarks(b),
	})

	// Registers unbookmark command.
	b.Router.RegisterCmd(&gumi.Command{
		Name:        "unbookmark",
		Group:       group,
		Aliases:     []string{"unfav", "unfavourite", "unfavorite"},
		Description: "Remove a bookmark by its ID or URL",
		Usage:       "bt!unfav <artwork ID or URL>",
		Example:     "bt!unfav 69",
		RateLimiter: gumi.NewRateLimiter(15 * time.Second),
		Exec:        unfav(b),
	})

	// Registers userset command.
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

// groups shows the full list of crosspost groups.
func groups(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		// Assigns current discord user.
		// Returns error if no user is found.
		user, err := b.Store.User(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		// Prepares new embed builder to list crosspost groups.
		locale := messages.UserGroupsEmbed(ctx.Event.Author.Username)
		eb := embeds.NewBuilder()

		// Builds embed title and description.
		eb.Title(locale.Title)
		eb.Description(locale.Description)

		// Iterates and constructs crosspost groups into groupList.
		var groupList [2][][2]string
		for _, group := range user.Groups {
			var category, parent, children string
			var list int8

			// Assigns crosspost group and pair variables
			if group.IsPair {
				category = locale.Pair
			} else {
				category = locale.Group
				parent = fmt.Sprintf("**%v: **", locale.Parent)
				children = fmt.Sprintf("**%v: **", locale.Children)
				list = 1
			}

			// Assigns field name and description.
			name := fmt.Sprintf("%v «%v»", category, group.Name)
			desc := fmt.Sprintf("%v%v\n%v%v",
				parent,
				fmt.Sprintf("<#%v> | `%v`", group.Parent, group.Parent),
				children,
				strings.Join(arrays.Map(group.Children, func(s string) string {
					return fmt.Sprintf("<#%v> | `%v`", s, s)
				}), "\n"),
			)

			// Appends to groupList.
			groupList[list] = append(groupList[list], [2]string{name, desc})
		}

		// Builds crosspost groups into respective fields.
		for _, list := range groupList {
			for _, group := range list {
				eb.AddField(group[0], group[1])
			}
		}

		// Returns embed list of crosspost groups.
		return ctx.ReplyEmbed(eb.Finalize())
	}
}

// newgroup creates a new crosspost group or pair.
// A crosspost group is given a name and parent channel.
// A crosspost pair is also given a crosspost channel.
func newgroup(b *bot.Bot, ispair bool) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		// Checks if Boe Tea command is correct.
		//
		// Crosspost group command suffices with two (2) arguments.
		// Crosspost pair command suffices with three (3) arguments.
		if (ispair && ctx.Args.Len() != 3) || ctx.Args.Len() < 2 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		// Assigns current discord user.
		// Returns error if no user is found.
		user, err := b.Store.User(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		// Assigns name of crosspost group or pair.
		name := ctx.Args.Get(0).Raw
		// Assigns name of parent channel.
		// Returns error if no channel exists.
		parent := strings.Trim(ctx.Args.Get(1).Raw, "<#>")
		if _, err := ctx.Session.Channel(parent); err != nil {
			return messages.ErrChannelNotFound(err, parent)
		}

		// Assigns a crosspost channel when crosspost group is a pair.
		// If group is not a pair leave empty.
		children := []string{}
		if ispair {
			// Assigns channelID of crosspost channel.
			child := strings.Trim(ctx.Args.Get(2).Raw, "<#>")

			// Returns discord channel from channelID.
			// If no discord channel is found return error.
			ch, err := ctx.Session.Channel(child)
			if err != nil {
				return messages.ErrChannelNotFound(err, child)
			}

			// Checks if discord channel is a text channel.
			if ch.Type != discordgo.ChannelTypeGuildText {
				return messages.ErrIncorrectCmd(ctx.Command)
			}

			// Checks if crosspost channel is not parent channel.
			if parent == child {
				return messages.ErrIncorrectCmd(ctx.Command)
			}

			// Checks if crosspost channel is not parent of an existing group.
			if _, ok := user.FindGroupByParent(child); ok {
				return messages.ErrInsertGroup(name, child)
			}

			// Appends crosspost channel to children.
			children = append(children, child)
		}

		// Creates crosspost group when checks suffice.
		_, err = b.Store.CreateCrosspostGroup(context.Background(), user.ID, &store.Group{
			Name:     name,
			Parent:   parent,
			Children: children,
			IsPair:   ispair,
		})

		// Returns error if any mongoDB error is raised.
		if err != nil {
			if err = errMongoDB(err, messages.ErrInsertGroup(name, parent)); err != nil {
				return err
			}
		}

		// Assign success message for crosspost group or pair.
		var msg string
		if ispair {
			msg = messages.UserCreatePairSuccess(name, parent, children[0])
		} else {
			msg = messages.UserCreateGroupSuccess(name, parent)
		}

		// Returns success message embed.
		return successMessage(ctx, msg)
	}
}

// delgroup deletes a crosspost group.
func delgroup(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		// Checks if Boe Tea command is correct.
		if ctx.Args.Len() < 1 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		// Assigns current discord user.
		// Returns error if no user is found.
		user, err := b.Store.User(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		// Assigns name of crosspost group.
		name := ctx.Args.Get(0).Raw

		// Deletes crosspost group when checks suffice.
		_, err = b.Store.DeleteCrosspostGroup(context.Background(), user.ID, name)

		// Returns error if any mongoDB error is raised.
		if err != nil {
			if err = errMongoDB(err, messages.ErrDeleteGroup(name)); err != nil {
				return err
			}
		}

		// Returns success message embed.
		return successMessage(ctx, fmt.Sprintf("Removed a group named `%v`", name))
	}
}

// push adds one or more crosspost channels to a group.
func push(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		// Checks if Boe Tea command is correct.
		if ctx.Args.Len() < 2 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		// Assigns current discord user.
		// Returns error if no user is found.
		user, err := b.Store.User(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		// Assigns name of crosspost group.
		name := ctx.Args.Get(0).Raw
		ctx.Args.Remove(0)

		// Assigns and checks if crosspost group exists.
		group, ok := user.FindGroupByName(name)
		if !ok {
			return messages.ErrUserPushFail(name)
		}

		// Checks if group is not a pair.
		if group.IsPair {
			return messages.ErrUserPairFail(name)
		}

		// Iterates crosspost channels to be inserted into group.
		inserted := make([]string, 0, ctx.Args.Len())
		for _, arg := range ctx.Args.Arguments {
			// Assigns channelID of crosspost channel.
			channelID := strings.Trim(arg.Raw, "<#>")

			// Returns discord channel from channelID.
			// Returns error if no discord channel found.
			ch, err := ctx.Session.Channel(channelID)
			if err != nil {
				return messages.ErrChannelNotFound(err, channelID)
			}

			// Checks if discord channel is a text channel.
			if ch.Type != discordgo.ChannelTypeGuildText {
				continue
			}

			// Checks if crosspost channel is not a parent channel of group.
			if group.Parent == channelID {
				continue
			}

			// Checks if crosspost channel is already a channel of this group.
			if arrays.Any(group.Children, channelID) {
				continue
			}

			// Adds crosspost channel to group when checks suffice.
			_, err = b.Store.AddCrosspostChannel(
				context.Background(),
				user.ID,
				name,
				channelID,
			)

			// Returns error if any mongoDB error is raised.
			if err != nil {
				if err = errMongoDB(err); err != nil {
					return err
				} else {
					continue
				}
			}

			// Assigns crosspost channel as being inserted.
			inserted = append(inserted, channelID)
		}

		// Checks if any crosspost channel has been inserted into group.
		// Returns error message if no channel has been inserted.
		if len(inserted) == 0 {
			return messages.ErrUserPushFail(name)
		}

		// Returns success message embed.
		return successMessage(ctx, messages.UserPushSuccess(name, inserted))
	}
}

// remove removes one or more crosspost channels from a group.
func remove(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		// Checks if Boe Tea command is correct.
		if ctx.Args.Len() < 2 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		// Assigns current discord user.
		// Returns error if no user is found.
		user, err := b.Store.User(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		// Assigns name of crosspost group or pair.
		name := ctx.Args.Get(0).Raw
		ctx.Args.Remove(0)

		// Assigns and checks if crosspost group exists.
		group, ok := user.FindGroupByName(name)
		if !ok {
			return messages.ErrUserRemoveFail(name)
		}

		// Checks if crosspost group is not a pair.
		if group.IsPair {
			return messages.ErrUserPairFail(name)
		}

		// Iterates crosspost channels to be removed from group.
		removed := make([]string, 0, ctx.Args.Len())
		for _, arg := range ctx.Args.Arguments {
			// Assigns channelID of crosspost channel.
			channelID := strings.Trim(arg.Raw, "<#>")

			// Checks if crosspost channel exists in group.
			if !arrays.Any(group.Children, channelID) {
				continue
			}

			// Removes crosspost channel from group when checks suffice.
			_, err = b.Store.DeleteCrosspostChannel(
				context.Background(),
				user.ID,
				name,
				channelID,
			)

			// Returns error if any mongoDB error is raised.
			if err != nil {
				if err = errMongoDB(err); err != nil {
					return err
				} else {
					continue
				}
			}

			// Assigns crosspost channel as removed.
			removed = append(removed, channelID)
		}

		// Checks if any crosspost channel has been removed.
		// Returns error message if no channel has been removed.
		if len(removed) == 0 {
			return messages.ErrUserRemoveFail(name)
		}

		// Returns success message embed.
		return successMessage(ctx, messages.UserRemoveSuccess(name, removed))
	}
}

// rename changes the name of a crosspost group.
func rename(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		// Checks if Boe Tea command is correct.
		if ctx.Args.Len() < 2 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		// Assigns current discord user.
		// Returns error if no user is found.
		user, err := b.Store.User(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		// Assigns group name and new name.
		cmd := "rename"
		src := ctx.Args.Get(0).Raw
		dest := ctx.Args.Get(1).Raw

		// Checks if crosspost group exists.
		_, ok := user.FindGroupByName(src)
		if !ok {
			return messages.ErrUserGroupNameFail(cmd, src, dest)
		}

		// Renames crosspost group when checks suffice.
		_, err = b.Store.RenameCrosspostGroup(context.Background(), user.ID, src, dest)
		if err != nil {
			return messages.ErrUserGroupNameFail(cmd, src, dest)
		}

		// Returns success message embed.
		return successMessage(ctx, messages.UserRenameSuccess(src, dest))
	}
}

// copygroup copies a crosspost group with a new name and parent channel.
func copygroup(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() < 3 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		// Assigns current discord user.
		// Returns error if no user is found.
		user, err := b.Store.User(context.Background(), ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		// Assigns crosspost group name and new name of copy.
		cmd := "copy"
		src := ctx.Args.Get(0).Raw
		dest := ctx.Args.Get(1).Raw

		// Assigns and checks if crosspost group exists.
		group, ok := user.FindGroupByName(src)
		if !ok {
			return messages.ErrUserGroupNameFail(cmd, src, dest)
		}

		// Checks if crosspost group is not a pair
		if group.IsPair {
			return messages.ErrUserPairFail(src)
		}

		// Checks if parent channel is already a parent of another group.
		parent := strings.Trim(ctx.Args.Get(2).Raw, "<#>")
		if _, ok := user.FindGroupByParent(parent); ok {
			return messages.ErrUserChannelAlreadyParent(parent)
		}

		// Assigns new crosspost group copy with new name and parent.
		newGroup := &store.Group{
			Name:   dest,
			Parent: parent,
			Children: arrays.Filter(group.Children, func(s string) bool {
				return s != parent
			}),
		}

		// Copies crosspost group when checks suffice.
		_, err = b.Store.CreateCrosspostGroup(context.Background(), user.ID, newGroup)
		if err != nil {
			return messages.ErrUserGroupNameFail(cmd, src, dest)
		}

		// Returns success message embed.
		return successMessage(ctx,
			messages.UserCopyGroupSuccess(src, dest, newGroup.Children),
		)
	}
}

func bookmarks(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		tctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		var (
			limit  int64 = 1
			order        = store.Descending
			sortBy       = store.ByTime
			args         = strings.Fields(ctx.Args.Raw)
			mode         = store.BookmarkFilterSafe
			filter       = store.ArtworkFilter{}
		)

		ch, err := ctx.Session.Channel(ctx.Event.ChannelID)
		if err != nil {
			return err
		}

		if ch.NSFW || ch.Type == discordgo.ChannelTypeDM {
			mode = store.BookmarkFilterAll
		}

		flagsMap, err := flags.FromArgs(args, flags.FlagTypeOrder, flags.FlagTypeMode)
		if err != nil {
			return err
		}

		for key, val := range flagsMap {
			switch key {
			case flags.FlagTypeOrder:
				order = val.(store.Order)
			case flags.FlagTypeMode:
				mode = val.(store.BookmarkFilter)
			}
		}

		bookmarks, err := b.Store.ListBookmarks(tctx, ctx.Event.Author.ID, mode, order)
		if err != nil {
			return err
		}

		if len(bookmarks) == 0 {
			return messages.ErrUserNoBookmarks(ctx.Event.Author.ID)
		}

		filter.IDs = make([]int, 0, limit)
		for _, bookmark := range bookmarks {
			if int64(len(filter.IDs)) == limit {
				break
			}

			filter.IDs = append(filter.IDs, bookmark.ArtworkID)
		}

		opts := store.ArtworkSearchOptions{
			Limit: limit,
			Order: order,
			Sort:  sortBy,
		}

		artworks, err := b.Store.SearchArtworks(tctx, filter, opts)
		if err != nil {
			return err
		}

		pages := make([]*discordgo.MessageEmbed, len(bookmarks))
		for ind, bookmark := range bookmarks {
			artwork := arrays.Find(artworks, func(a *store.Artwork) bool { return a.ID == bookmark.ArtworkID })
			if artwork == nil {
				break
			}

			page := artworkToEmbed(artwork, artwork.Images[0], ind, len(bookmarks))
			page.Fields = append(page.Fields, &discordgo.MessageEmbedField{
				Name:   "NSFW",
				Value:  strconv.FormatBool(bookmark.NSFW),
				Inline: true,
			})

			pages[ind] = page
		}

		wg := dgoutils.NewWidget(ctx.Session, ctx.Event.Author.ID, pages)
		wg.WithCallback(func(wa dgoutils.WidgetAction, i int) error {
			if wg.Pages[i] != nil {
				return nil
			}

			artwork, err := b.Store.Artwork(tctx, bookmarks[i].ArtworkID, "")
			if errors.Is(err, store.ErrArtworkNotFound) {
				eb := embeds.NewBuilder()
				eb.FailureTemplate("Artwork not found.").
					AddField("ID", strconv.Itoa(bookmarks[i].ArtworkID))

				wg.Pages[i] = eb.Finalize()

				_, err := b.Store.DeleteBookmark(tctx, bookmarks[i])
				if err != nil {
					return fmt.Errorf("failed to delete unknown bookmark: %w", err)
				}

				return nil
			}

			if err != nil {
				return err
			}

			page := artworkToEmbed(artwork, artwork.Images[0], i, len(bookmarks))
			page.Fields = append(page.Fields, &discordgo.MessageEmbedField{
				Name:   "NSFW",
				Value:  strconv.FormatBool(bookmarks[i].NSFW),
				Inline: true,
			})

			wg.Pages[i] = page
			return nil
		})
		return wg.Start(ctx.Event.ChannelID)
	}
}

func userset(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		switch {
		case ctx.Args.Len() == 0:
			return showUserProfile(b, ctx)
		case ctx.Args.Len() >= 2:
			return changeUserSettings(b, ctx)
		default:
			return messages.ErrIncorrectCmd(ctx.Command)
		}
	}
}

func showUserProfile(b *bot.Bot, ctx *gumi.Ctx) error {
	user, err := b.Store.User(context.Background(), ctx.Event.Author.ID)
	if err != nil {
		return err
	}

	bookmarks, err := b.Store.CountBookmarks(context.Background(), ctx.Event.Author.ID)
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
			locale.Bookmarks, bookmarks,
		),
	)

	return ctx.ReplyEmbed(eb.Finalize())
}

func changeUserSettings(b *bot.Bot, ctx *gumi.Ctx) error {
	user, err := b.Store.User(context.Background(), ctx.Event.Author.ID)
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
	case "ignore":
		new, err := parseBool(newSetting.Raw)
		if err != nil {
			return err
		}

		oldSettingEmbed = user.Ignore
		newSettingEmbed = new
		user.Ignore = new

	default:
		return messages.ErrUnknownUserSetting(settingName.Raw)
	}

	_, err = b.Store.UpdateUser(context.Background(), user)
	if err != nil {
		return err
	}

	eb := embeds.NewBuilder()
	eb.InfoTemplate("Successfully changed user setting.")
	eb.AddField("Setting name", settingName.Raw, true)
	eb.AddField("Old setting", fmt.Sprintf("%v", oldSettingEmbed), true)
	eb.AddField("New setting", fmt.Sprintf("%v", newSettingEmbed), true)

	return ctx.ReplyEmbed(eb.Finalize())
}

func unfav(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() == 0 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		var (
			id    int
			url   string
			err   error
			query = ctx.Args.Get(0).Raw
		)

		//If ID is not an integer assign query to the URL.
		if id, err = strconv.Atoi(query); err != nil {
			url = query
		}

		var artwork *store.Artwork
		if url != "" {
			artwork, err = b.Store.Artwork(context.Background(), 0, url)
			if err != nil {
				return messages.ErrArtworkNotFound(query)
			}

			id = artwork.ID
		}

		deleted, err := b.Store.DeleteBookmark(context.Background(), &store.Bookmark{UserID: ctx.Event.Author.ID, ArtworkID: id})
		if err != nil {
			return messages.ErrUserUnbookmarkFail(query, err)
		}

		if !deleted {
			return messages.ErrArtworkNotFound(query)
		}

		eb := embeds.NewBuilder()
		locale := messages.BookmarkRemovedEmbed()

		eb.Title(locale.Title).
			Description(locale.Description).
			AddField("ID", strconv.Itoa(artwork.ID), true).
			AddField("URL", messages.ClickHere(artwork.URL), true)

		if len(artwork.Images) > 0 {
			eb.Thumbnail(artwork.Images[0])
		}

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func artworkToEmbed(artwork *store.Artwork, image string, ind, length int) *discordgo.MessageEmbed {
	title := ""
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

	eb.AddField("ID", strconv.Itoa(artwork.ID), true).
		AddField("Author", artwork.Author, true).
		AddField("Bookmarks", strconv.Itoa(artwork.Favorites), true).
		AddField("URL", messages.ClickHere(artwork.URL)).
		Timestamp(artwork.CreatedAt)

	return eb.Finalize()
}

// errMongoDB returns an error if any mongoDB error is raised.
// If no error message is provided, errMongoDB will return the provided error or as nil.
func errMongoDB(err error, message ...error) error {
	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		if message != nil {
			return message[0]
		} else {
			return nil
		}
	default:
		return err
	}
}

// successMessage builds and returns success message embed.
func successMessage(ctx *gumi.Ctx, message string) error {
	eb := embeds.NewBuilder()
	eb.SuccessTemplate(message)
	return ctx.ReplyEmbed(eb.Finalize())
}
