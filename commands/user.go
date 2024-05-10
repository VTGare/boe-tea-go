package commands

import (
	"context"
	"errors"
	"fmt"
	"sort"
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
		Exec:        newGroup(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "newpair",
		Group:       group,
		Aliases:     []string{"addpair"},
		Description: "Creates a new crosspost pair.",
		Usage:       "bt!newpair <pair name> <first channel> <second channel>",
		Example:     "bt!newpair lewds #nsfw #nsfw-pics",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        newPair(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "delgroup",
		Group:       group,
		Description: "Deletes a crosspost group.",
		Usage:       "bt!delgroup <group name>",
		Example:     "bt!delgroup schooldays",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        delGroup(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "push",
		Group:       group,
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
		Name:        "editparent",
		Group:       group,
		Description: "Changes the parent channel of a crosspost group",
		Usage:       "bt!editparent <group name> <parent channel>",
		Example:     "bt!editparent cuteAnimeGirls #anime-pics",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        editParent(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "rename",
		Group:       group,
		Description: "Renames a crosspost group",
		Usage:       "bt!rename <from> <to>",
		Example:     "bt!rename cuteAnimeGirls AnimeGirls",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        rename(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "copygroup",
		Group:       group,
		Description: "Copies a crosspost group with a different parent channel",
		Usage:       "bt!copygroup <from> <to> <parent channel id>",
		Example:     "bt!copygroup sfw1 sfw2 #za-warudo",
		RateLimiter: gumi.NewRateLimiter(10 * time.Second),
		Exec:        copyGroup(b),
	})

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
		Exec:        userSet(b),
	})
}

// groups shows the full list of crosspost groups.
func groups(b *bot.Bot) func(ctx *gumi.Ctx) error {
	type groupData struct {
		Name        string
		Description string
	}

	type groupList struct {
		Pairs  []groupData
		Groups []groupData
	}

	return func(ctx *gumi.Ctx) error {
		user, err := initCommand(b, ctx, 0)
		if err != nil {
			return err
		}

		locale := messages.UserGroupsEmbed(ctx.Event.Author.Username)
		eb := embeds.NewBuilder()

		eb.Title(locale.Title)
		eb.Description(locale.Description)

		var groupList groupList
		for _, group := range user.Groups {
			var category, parent, children string
			if group.IsPair {
				category = locale.Pair
			} else {
				category = locale.Group
				parent = fmt.Sprintf("**%v:** %v\n",
					locale.Parent,
					fmt.Sprintf("<#%v> | `%v`", group.Parent, group.Parent),
				)
				children = fmt.Sprintf("**%v:** \n", locale.Children)
			}

			name := fmt.Sprintf("%v «%v»", category, group.Name)
			desc := fmt.Sprintf("%v %v %v",
				parent,
				children,
				strings.Join(arrays.Map(group.Children, func(s string) string {
					return fmt.Sprintf("<#%v> | `%v`", s, s)
				}), "\n"),
			)

			if group.IsPair {
				groupList.Pairs = append(groupList.Pairs, groupData{name, desc})
			} else {
				groupList.Groups = append(groupList.Groups, groupData{name, desc})
			}
		}

		for _, pair := range groupList.Pairs {
			eb.AddField(pair.Name, pair.Description)
		}

		for _, group := range groupList.Groups {
			eb.AddField(group.Name, group.Description)
		}

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

// newGroup creates a new crosspost group.
func newGroup(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		user, err := initCommand(b, ctx, 2)
		if err != nil {
			return err
		}

		// Name of crosspost group.
		name := ctx.Args.Get(0).Raw
		parent := dgoutils.Trimmer(ctx, 1)
		if _, err := ctx.Session.Channel(parent); err != nil {
			return messages.ErrChannelNotFound(err, parent)
		}

		if _, ok := user.FindGroupByName(name); ok {
			return messages.ErrGroupAlreadyExists(name)
		}

		// Checks if parent is already used.
		if _, ok := user.FindGroup(parent); ok {
			return messages.ErrNewGroup(name, parent)
		}

		_, err = b.Store.CreateCrosspostGroup(b.Context, user.ID, &store.Group{
			Name:     name,
			Parent:   parent,
			Children: []string{},
			IsPair:   false,
		})

		if err := handleStoreError(err, messages.ErrNewGroup(name, parent)); err != nil {
			return err
		}

		return successMessage(ctx, messages.UserCreateGroupSuccess(name, parent))
	}
}

// newPair creates a new crosspost pair.
func newPair(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		user, err := initCommand(b, ctx, 3)
		if err != nil {
			return err
		}

		// Name of crosspost pair.
		name := ctx.Args.Get(0).Raw
		var children []string
		children = append(children,
			dgoutils.Trimmer(ctx, 1),
			dgoutils.Trimmer(ctx, 2),
		)

		// Checks if crosspost channel is not parent channel.
		if children[0] == children[1] {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		if _, ok := user.FindGroupByName(name); ok {
			return messages.ErrGroupAlreadyExists(name)
		}

		for _, child := range children {
			ch, err := ctx.Session.Channel(child)
			if err != nil {
				return messages.ErrChannelNotFound(err, child)
			}

			if ch.Type != discordgo.ChannelTypeGuildText {
				return messages.ErrIncorrectCmd(ctx.Command)
			}

			if _, ok := user.FindGroup(child); ok {
				return messages.ErrNewPair(name, children)
			}
		}

		sort.Strings(children)
		_, err = b.Store.CreateCrosspostPair(b.Context, user.ID, &store.Group{
			Name:     name,
			Children: children,
			IsPair:   true,
		})

		if err := handleStoreError(err, messages.ErrNewPair(name, children)); err != nil {
			return err
		}

		return successMessage(ctx, messages.UserCreatePairSuccess(name, children))
	}
}

// delGroup deletes a crosspost group.
func delGroup(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		user, err := initCommand(b, ctx, 1)
		if err != nil {
			return err
		}

		// Name of crosspost group.
		name := ctx.Args.Get(0).Raw

		_, err = b.Store.DeleteCrosspostGroup(b.Context, user.ID, name)
		if err := handleStoreError(err, messages.ErrDeleteGroup(name)); err != nil {
			return err
		}

		return successMessage(ctx, fmt.Sprintf("Removed a group named `%v`", name))
	}
}

// push adds one or more crosspost channels to a group.
func push(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		user, err := initCommand(b, ctx, 2)
		if err != nil {
			return err
		}

		// Name of crosspost group.
		name := ctx.Args.Get(0).Raw
		ctx.Args.Remove(0)

		group, ok := user.FindGroupByName(name)
		if !ok {
			return messages.ErrGroupExistFail(name)
		}

		if group.IsPair {
			return messages.ErrUserPairFail(name)
		}

		inserted := make([]string, 0, ctx.Args.Len())
		for arg := range ctx.Args.Arguments {
			channelID := dgoutils.Trimmer(ctx, arg)
			ch, err := ctx.Session.Channel(channelID)
			if err != nil {
				return messages.ErrChannelNotFound(err, channelID)
			}

			// Only accept guild text channels.
			if ch.Type != discordgo.ChannelTypeGuildText {
				continue
			}

			if group.Parent == channelID {
				continue
			}

			if _, ok := user.FindGroup(channelID); ok {
				continue
			}

			if arrays.Any(group.Children, channelID) {
				continue
			}

			_, err = b.Store.AddCrosspostChannel(
				b.Context,
				user.ID,
				name,
				channelID,
			)

			if err := handleStoreError(err); err != nil {
				return err
			}

			inserted = append(inserted, channelID)
		}

		if len(inserted) == 0 {
			return messages.ErrUserPushFail(name)
		}

		return successMessage(ctx, messages.UserPushSuccess(name, inserted))
	}
}

// remove deletes one or more crosspost channels from a group.
func remove(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		user, err := initCommand(b, ctx, 2)
		if err != nil {
			return err
		}

		// Name of crosspost group or pair.
		name := ctx.Args.Get(0).Raw
		ctx.Args.Remove(0)

		group, ok := user.FindGroupByName(name)
		if !ok {
			return messages.ErrGroupExistFail(name)
		}

		if group.IsPair {
			return messages.ErrUserPairFail(name)
		}

		removed := make([]string, 0, ctx.Args.Len())
		for arg := range ctx.Args.Arguments {
			channelID := dgoutils.Trimmer(ctx, arg)

			if !arrays.Any(group.Children, channelID) {
				continue
			}

			_, err = b.Store.DeleteCrosspostChannel(
				b.Context,
				user.ID,
				name,
				channelID,
			)

			if err := handleStoreError(err); err != nil {
				return err
			}

			removed = append(removed, channelID)
		}

		if len(removed) == 0 {
			return messages.ErrUserRemoveFail(name)
		}

		return successMessage(ctx, messages.UserRemoveSuccess(name, removed))
	}
}

// editParent changes the parent channel of a group
func editParent(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		user, err := initCommand(b, ctx, 2)
		if err != nil {
			return err
		}

		// Name of crosspost group
		name := ctx.Args.Get(0).Raw
		group, ok := user.FindGroupByName(name)
		if !ok {
			return messages.ErrGroupExistFail(name)
		}

		dest := dgoutils.Trimmer(ctx, 1)
		if _, err := ctx.Session.Channel(dest); err != nil {
			return messages.ErrChannelNotFound(err, dest)
		}

		if group.IsPair {
			return messages.ErrUserPairFail(name)
		}

		if _, ok := user.FindGroup(dest); ok {
			return messages.ErrGroupAlreadyExists(dest)
		}

		if arrays.Any(group.Children, dest) {
			return messages.ErrUserEditParentFail(group.Parent, dest)
		}

		_, err = b.Store.EditCrosspostParent(b.Context, user.ID, name, dest)
		if err := handleStoreError(err, messages.ErrUserEditParentFail(group.Parent, dest)); err != nil {
			return err
		}

		return successMessage(ctx, messages.UserEditParentSuccess(group.Parent, dest))
	}
}

// rename changes the name of a group
func rename(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		user, err := initCommand(b, ctx, 2)
		if err != nil {
			return err
		}

		// Group name (0) and new name (1)
		var (
			cmd  = "rename"
			src  = ctx.Args.Get(0).Raw
			dest = ctx.Args.Get(1).Raw
		)

		if _, ok := user.FindGroupByName(src); !ok {
			return messages.ErrGroupExistFail(src)
		}

		_, err = b.Store.RenameCrosspostGroup(b.Context, user.ID, src, dest)
		if err != nil {
			return messages.ErrUserEditGroupFail(cmd, src, dest)
		}

		return successMessage(ctx, messages.UserRenameSuccess(src, dest))
	}
}

// copyGroup copies a crosspost group with a new name and parent channel.
func copyGroup(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		user, err := initCommand(b, ctx, 3)
		if err != nil {
			return err
		}

		// Source group name (0), destination group name (1)
		var (
			cmd  = "copy"
			src  = ctx.Args.Get(0).Raw
			dest = ctx.Args.Get(1).Raw
		)

		group, ok := user.FindGroupByName(src)
		if !ok {
			return messages.ErrGroupExistFail(src)
		}

		if group.IsPair {
			return messages.ErrUserPairFail(src)
		}

		parent := dgoutils.Trimmer(ctx, 2)
		if _, ok := user.FindGroup(parent); ok {
			return messages.ErrUserChannelAlreadyParent(parent)
		}

		newGroup := &store.Group{
			Name:   dest,
			Parent: parent,
			Children: arrays.Filter(group.Children, func(s string) bool {
				return s != parent
			}),
		}

		_, err = b.Store.CreateCrosspostGroup(b.Context, user.ID, newGroup)
		if err != nil {
			return messages.ErrUserEditGroupFail(cmd, src, dest)
		}

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

func userSet(b *bot.Bot) func(ctx *gumi.Ctx) error {
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
	user, err := b.Store.User(b.Context, ctx.Event.Author.ID)
	if err != nil {
		return err
	}

	bookmarks, err := b.Store.CountBookmarks(b.Context, ctx.Event.Author.ID)
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
			locale.CrossPost, messages.FormatBool(user.Crosspost),
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
	user, err := b.Store.User(b.Context, ctx.Event.Author.ID)
	if err != nil {
		return err
	}

	var (
		settingName     = ctx.Args.Get(0)
		newSetting      = ctx.Args.Get(1)
		newSettingEmbed any
		oldSettingEmbed any
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

	_, err = b.Store.UpdateUser(b.Context, user)
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

		// If ID is not an integer assign query to the URL.
		if id, err = strconv.Atoi(query); err != nil {
			url = query
		}

		var artwork *store.Artwork
		if url != "" {
			artwork, err = b.Store.Artwork(b.Context, 0, url)
			if err != nil {
				return messages.ErrArtworkNotFound(query)
			}

			id = artwork.ID
		}

		deleted, err := b.Store.DeleteBookmark(b.Context, &store.Bookmark{UserID: ctx.Event.Author.ID, ArtworkID: id})
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

func initCommand(b *bot.Bot, ctx *gumi.Ctx, argsLen int) (*store.User, error) {
	if err := dgoutils.InitCommand(ctx, argsLen); err != nil {
		return nil, err
	}

	return b.Store.User(b.Context, ctx.Event.Author.ID)
}

// handleStoreError returns an error if any store error is raised.
// If no error message is provided, handleStoreError will return the provided error or as nil.
func handleStoreError(err error, message ...error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, mongo.ErrNoDocuments):
		if message != nil {
			return message[0]
		}

		return nil
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
