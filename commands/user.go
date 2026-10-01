package commands

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/internal/widget"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/router"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"
	"github.com/bwmarrin/discordgo"
	"github.com/julien040/go-ternary"
)

// userGroup registers user group commands.
func userGroup(b *bot.Bot) []*router.Command {
	userCooldown := func() *router.Cooldown {
		return router.NewCooldown(router.CooldownUser, 1, 10*time.Second)
	}

	groupCommands := []*router.Command{
		{
			Name:        "list",
			Description: "Shows all crosspost groups.",
			Examples:    []string{"groups list"},
			Handler:     groups(b),
		},
		{
			Name:        "create",
			Description: "Creates a new crosspost group.",
			Options: []*router.Option{
				router.String("name", "Group name").Require(),
				router.Channel("parent", "Parent channel").Require(),
			},
			Examples: []string{"groups create lewds #nsfw"},
			Handler:  newGroup(b),
		},
		{
			Name:        "pair",
			Description: "Creates a new crosspost pair.",
			Options: []*router.Option{
				router.String("name", "Pair name").Require(),
				router.Channel("first", "First channel").Require().
					WithChannelTypes(discordgo.ChannelTypeGuildText),
				router.Channel("second", "Second channel").Require().
					WithChannelTypes(discordgo.ChannelTypeGuildText),
			},
			Examples: []string{"groups pair lewds #nsfw #nsfw-pics"},
			Handler:  newPair(b),
		},
		{
			Name:        "delete",
			Description: "Deletes a crosspost group.",
			Options: []*router.Option{
				router.String("name", "Group name").Require(),
			},
			Examples: []string{"groups delete schooldays"},
			Handler:  delGroup(b),
		},
		{
			Name:        "add",
			Description: "Adds channels to a crosspost group.",
			Options: []*router.Option{
				router.String("group", "Group name").Require(),
				router.String("channels", "Channels to add").Require().Greedy(),
			},
			Examples: []string{"groups add myCoolGroup #coolchannel #coolerchannel"},
			Handler:  push(b),
		},
		{
			Name:        "remove",
			Description: "Removes channels from a crosspost group.",
			Options: []*router.Option{
				router.String("group", "Group name").Require(),
				router.String("channels", "Channels to remove").Require().Greedy(),
			},
			Examples: []string{"groups remove cuteAnimeGirls #nsfw-channel #cat-pics"},
			Handler:  remove(b),
		},
		{
			Name:        "parent",
			Description: "Changes the parent channel of a crosspost group.",
			Options: []*router.Option{
				router.String("group", "Group name").Require(),
				router.Channel("parent", "New parent channel").Require(),
			},
			Examples: []string{"groups parent cuteAnimeGirls #anime-pics"},
			Handler:  editParent(b),
		},
		{
			Name:        "rename",
			Description: "Renames a crosspost group.",
			Options: []*router.Option{
				router.String("from", "Current name").Require(),
				router.String("to", "New name").Require(),
			},
			Examples: []string{"groups rename cuteAnimeGirls AnimeGirls"},
			Handler:  rename(b),
		},
		{
			Name:        "copy",
			Description: "Copies a crosspost group with a different parent channel.",
			Options: []*router.Option{
				router.String("from", "Source group").Require(),
				router.String("to", "New group").Require(),
				router.Channel("parent", "New parent channel").Require(),
			},
			Examples: []string{"groups copy sfw1 sfw2 #za-warudo"},
			Handler:  copyGroup(b),
		},
	}

	bookmarkCommands := []*router.Command{
		{
			Name:        "list",
			Description: "Shows your bookmarks.",
			Cooldown:    userCooldown(),
			Options: []*router.Option{
				router.String("sort", "How to sort bookmarks").WithChoices(
					router.Choice{Name: "time", Value: "time"},
					router.Choice{Name: "popularity", Value: "popularity"},
				),
				router.String("order", "Sort direction").WithChoices(
					router.Choice{Name: "asc", Value: "asc"},
					router.Choice{Name: "desc", Value: "desc"},
				),
				router.String("mode", "Which bookmarks to show").WithChoices(
					router.Choice{Name: "all", Value: "all"},
					router.Choice{Name: "sfw", Value: "sfw"},
					router.Choice{Name: "nsfw", Value: "nsfw"},
				),
				router.String("during", "Only include recent bookmarks").WithChoices(
					router.Choice{Name: "day", Value: "day"},
					router.Choice{Name: "week", Value: "week"},
					router.Choice{Name: "month", Value: "month"},
				),
			},
			Examples: []string{"bookmarks list popularity asc"},
			Handler:  bookmarks(b),
		},
		{
			Name:        "remove",
			Description: "Removes a bookmark by its ID or URL.",
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 15*time.Second),
			Options: []*router.Option{
				router.String("query", "Artwork ID or URL").Require(),
			},
			Examples: []string{"bookmarks remove 69"},
			Handler:  unfav(b),
		},
	}

	profileCommands := []*router.Command{
		{
			Name:        "show",
			Description: "Shows your settings and stats.",
			Examples:    []string{"profile"},
			Handler:     showUserProfile(b),
		},
		userToggle(b, "dm", "Sends you a DM when you add or remove a bookmark."),
		userToggle(b, "crosspost", "Crossposts artworks you share to your crosspost groups."),
		userToggle(b, "ignore", "Makes Boe Tea ignore artwork links you post."),
	}

	for _, sub := range slices.Concat(groupCommands, profileCommands) {
		sub.Cooldown = userCooldown()
	}

	return []*router.Command{
		{
			Name:        "groups",
			Category:    "User",
			Aliases:     []string{"ls", "list"},
			Description: "Shows or changes your crosspost groups.",
			Default:     "list",
			Subcommands: groupCommands,
		},
		{
			Name:        "bookmarks",
			Category:    "User",
			Aliases:     []string{"favorites", "favourites", "favs"},
			Description: "Shows or removes your bookmarks.",
			Default:     "list",
			Subcommands: bookmarkCommands,
		},
		{
			Name:        "profile",
			Category:    "User",
			Aliases:     []string{"userset"},
			Description: "Shows or changes your settings.",
			Default:     "show",
			Subcommands: profileCommands,
		},
		oldPrefixCommand(bookmarkCommands[1], "unbookmark", "unfav", "unfavourite", "unfavorite"),
	}
}

// oldPrefixCommand copies a subcommand into a hidden, prefix-only root
// command under the name it had before it moved into a group.
func oldPrefixCommand(sub *router.Command, name string, aliases ...string) *router.Command {
	return &router.Command{
		Name:         name,
		Category:     "User",
		Aliases:      aliases,
		Description:  sub.Description,
		Cooldown:     sub.Cooldown,
		Options:      sub.Options,
		Handler:      sub.Handler,
		DisableSlash: true,
		Hidden:       true,
	}
}

func userToggle(b *bot.Bot, setting, description string) *router.Command {
	return &router.Command{
		Name:        setting,
		Description: description,
		Options: []*router.Option{
			router.Boolean("value", "On or off").Require(),
		},
		Examples: []string{"profile " + setting + " off"},
		Handler: func(ctx *router.Context) error {
			return changeUserSettings(b, ctx, setting, ctx.Options.Bool("value"))
		},
	}
}

// groups shows the full list of crosspost groups.
func groups(b *bot.Bot) router.Handler {
	type groupData struct {
		Name        string
		Description string
	}

	type groupList struct {
		Pairs  []groupData
		Groups []groupData
	}

	return func(ctx *router.Context) error {
		user, err := initUser(b, ctx)
		if err != nil {
			return err
		}

		locale := messages.UserGroupsEmbed(ctx.Author().Username)
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
				parent = fmt.Sprintf(
					"**%v:** %v\n",
					locale.Parent,
					fmt.Sprintf("<#%v> | `%v`", group.Parent, group.Parent),
				)
				children = fmt.Sprintf("**%v:** \n", locale.Children)
			}

			name := fmt.Sprintf("%v «%v»", category, group.Name)
			desc := fmt.Sprintf(
				"%v %v %v",
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

		return ctx.Reply(router.Embed(eb.Finalize()))
	}
}

// newGroup creates a new crosspost group.
func newGroup(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		user, err := initUser(b, ctx)
		if err != nil {
			return err
		}

		name := ctx.Options.String("name")
		parent := ctx.Options.ID("parent")
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

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 5*time.Second)
		defer cancel()

		_, err = b.Store.CreateCrosspostGroup(reqCtx, user.ID, &store.Group{
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
func newPair(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		user, err := initUser(b, ctx)
		if err != nil {
			return err
		}

		name := ctx.Options.String("name")
		children := []string{ctx.Options.ID("first"), ctx.Options.ID("second")}

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

		slices.Sort(children)

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 5*time.Second)
		defer cancel()

		_, err = b.Store.CreateCrosspostPair(reqCtx, user.ID, &store.Group{
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
func delGroup(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		user, err := initUser(b, ctx)
		if err != nil {
			return err
		}

		name := ctx.Options.String("name")

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 5*time.Second)
		defer cancel()

		_, err = b.Store.DeleteCrosspostGroup(reqCtx, user.ID, name)
		if err := handleStoreError(err, messages.ErrDeleteGroup(name)); err != nil {
			return err
		}

		return successMessage(ctx, fmt.Sprintf("Removed a group named `%v`", name))
	}
}

// push adds one or more crosspost channels to a group.
func push(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		user, err := initUser(b, ctx)
		if err != nil {
			return err
		}

		name := ctx.Options.String("group")
		group, ok := user.FindGroupByName(name)
		if !ok {
			return messages.ErrGroupExistFail(name)
		}

		if group.IsPair {
			return messages.ErrUserPairFail(name)
		}

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 15*time.Second)
		defer cancel()

		targets := strings.Fields(ctx.Options.String("channels"))
		inserted := make([]string, 0, len(targets))
		for _, arg := range targets {
			channelID := dgoutils.TrimmerRaw(arg)
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

			if slices.Contains(group.Children, channelID) {
				continue
			}

			_, err = b.Store.AddCrosspostChannel(
				reqCtx,
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
func remove(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		user, err := initUser(b, ctx)
		if err != nil {
			return err
		}

		name := ctx.Options.String("group")
		group, ok := user.FindGroupByName(name)
		if !ok {
			return messages.ErrGroupExistFail(name)
		}

		if group.IsPair {
			return messages.ErrUserPairFail(name)
		}

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 15*time.Second)
		defer cancel()

		targets := strings.Fields(ctx.Options.String("channels"))
		removed := make([]string, 0, len(targets))
		for _, arg := range targets {
			channelID := dgoutils.TrimmerRaw(arg)

			if !slices.Contains(group.Children, channelID) {
				continue
			}

			_, err = b.Store.DeleteCrosspostChannel(
				reqCtx,
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
func editParent(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		user, err := initUser(b, ctx)
		if err != nil {
			return err
		}

		name := ctx.Options.String("group")
		group, ok := user.FindGroupByName(name)
		if !ok {
			return messages.ErrGroupExistFail(name)
		}

		dest := ctx.Options.ID("parent")
		if _, err := ctx.Session.Channel(dest); err != nil {
			return messages.ErrChannelNotFound(err, dest)
		}

		if group.IsPair {
			return messages.ErrUserPairFail(name)
		}

		if _, ok := user.FindGroup(dest); ok {
			return messages.ErrGroupAlreadyExists(dest)
		}

		if slices.Contains(group.Children, dest) {
			return messages.ErrUserEditParentFail(group.Parent, dest)
		}

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 15*time.Second)
		defer cancel()

		_, err = b.Store.EditCrosspostParent(reqCtx, user.ID, name, dest)
		if err := handleStoreError(err, messages.ErrUserEditParentFail(group.Parent, dest)); err != nil {
			return err
		}

		return successMessage(ctx, messages.UserEditParentSuccess(group.Parent, dest))
	}
}

// rename changes the name of a group
func rename(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		user, err := initUser(b, ctx)
		if err != nil {
			return err
		}

		var (
			cmd  = "rename"
			src  = ctx.Options.String("from")
			dest = ctx.Options.String("to")
		)

		if _, ok := user.FindGroupByName(src); !ok {
			return messages.ErrGroupExistFail(src)
		}

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 5*time.Second)
		defer cancel()

		_, err = b.Store.RenameCrosspostGroup(reqCtx, user.ID, src, dest)
		if err != nil {
			return messages.ErrUserEditGroupFail(cmd, src, dest)
		}

		return successMessage(ctx, messages.UserRenameSuccess(src, dest))
	}
}

// copyGroup copies a crosspost group with a new name and parent channel.
func copyGroup(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		user, err := initUser(b, ctx)
		if err != nil {
			return err
		}

		var (
			cmd  = "copy"
			src  = ctx.Options.String("from")
			dest = ctx.Options.String("to")
		)

		group, ok := user.FindGroupByName(src)
		if !ok {
			return messages.ErrGroupExistFail(src)
		}

		if group.IsPair {
			return messages.ErrUserPairFail(src)
		}

		parent := ctx.Options.ID("parent")
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

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 5*time.Second)
		defer cancel()

		_, err = b.Store.CreateCrosspostGroup(reqCtx, user.ID, newGroup)
		if err != nil {
			return messages.ErrUserEditGroupFail(cmd, src, dest)
		}

		return successMessage(
			ctx,
			messages.UserCopyGroupSuccess(src, dest, newGroup.Children),
		)
	}
}

func bookmarks(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		reqCtx, cancel := context.WithTimeout(ctx.Context(), 5*time.Second)
		defer cancel()

		var (
			limit  int64 = 1
			order        = store.Descending
			sortBy       = store.ByTime
			mode         = store.BookmarkFilterSafe
			filter       = store.ArtworkFilter{}
		)

		ch, err := ctx.Session.Channel(ctx.ChannelID())
		if err != nil {
			return err
		}

		if ch.NSFW || ch.Type == discordgo.ChannelTypeDM {
			mode = store.BookmarkFilterAll
		}

		if ctx.Options.String("order") == "asc" {
			order = store.Ascending
		}

		if ctx.Options.String("sort") == "popularity" {
			sortBy = store.ByPopularity
		}

		switch ctx.Options.String("mode") {
		case "all":
			mode = store.BookmarkFilterAll
		case "sfw":
			mode = store.BookmarkFilterSafe
		case "nsfw":
			mode = store.BookmarkFilterUnsafe
		}

		filter.Time = parseDuring(ctx.Options.String("during"))

		bookmarks, err := b.Store.ListBookmarks(reqCtx, ctx.AuthorID(), mode, order)
		if err != nil {
			return err
		}

		if len(bookmarks) == 0 {
			return messages.ErrUserNoBookmarks(ctx.AuthorID())
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

		found, err := b.Store.SearchArtworks(reqCtx, filter, opts)
		if err != nil {
			return err
		}

		pages := make([]*discordgo.MessageEmbed, len(bookmarks))
		for ind, bookmark := range bookmarks {
			artwork := arrays.Find(found, func(a *store.Artwork) bool { return a.ID == bookmark.ArtworkID })
			if artwork == nil {
				break
			}

			page := artworkToEmbed(artwork, firstArtworkImage(artwork), ind, len(bookmarks))
			page.Fields = append(page.Fields, &discordgo.MessageEmbedField{
				Name:   "NSFW",
				Value:  strconv.FormatBool(bookmark.NSFW),
				Inline: true,
			})

			pages[ind] = page
		}

		w := widget.New(ctx.AuthorID(), pages)
		w.WithCallback(func(_ widget.Action, i int) error {
			if w.Pages[i] != nil {
				return nil
			}

			return loadBookmarkPage(context.WithoutCancel(reqCtx), b, bookmarks, i, w.Pages)
		})

		return serveWidget(ctx, b, w)
	}
}

func loadBookmarkPage(ctx context.Context, b *bot.Bot, bookmarks []*store.Bookmark, i int, pages []*discordgo.MessageEmbed) error {
	artwork, err := b.Store.Artwork(ctx, bookmarks[i].ArtworkID, "")
	if errors.Is(err, store.ErrArtworkNotFound) {
		eb := embeds.NewBuilder()
		eb.FailureTemplate("Artwork not found.").
			AddField("ID", strconv.Itoa(bookmarks[i].ArtworkID))

		pages[i] = eb.Finalize()

		_, err := b.Store.DeleteBookmark(ctx, bookmarks[i])
		if err != nil {
			return fmt.Errorf("failed to delete unknown bookmark: %w", err)
		}

		return nil
	}

	if err != nil {
		return err
	}

	if artwork == nil {
		return messages.ErrArtworkNotFound(strconv.Itoa(bookmarks[i].ArtworkID))
	}

	page := artworkToEmbed(artwork, firstArtworkImage(artwork), i, len(bookmarks))
	page.Fields = append(page.Fields, &discordgo.MessageEmbedField{
		Name:   "NSFW",
		Value:  strconv.FormatBool(bookmarks[i].NSFW),
		Inline: true,
	})

	pages[i] = page
	return nil
}

func showUserProfile(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		reqCtx, cancel := context.WithTimeout(ctx.Context(), 20*time.Second)
		defer cancel()

		user, err := b.Store.User(reqCtx, ctx.AuthorID())
		if err != nil {
			return err
		}

		bookmarks, err := b.Store.CountBookmarks(reqCtx, ctx.AuthorID())
		if err != nil {
			return err
		}

		locale := messages.UserProfileEmbed(ctx.Author().Username)
		eb := embeds.NewBuilder()
		eb.Title(locale.Title)
		eb.Thumbnail(ctx.Author().AvatarURL(""))

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

		return ctx.Reply(router.Embed(eb.Finalize()))
	}
}

func changeUserSettings(b *bot.Bot, ctx *router.Context, settingName string, value bool) error {
	reqCtx, cancel := context.WithTimeout(ctx.Context(), 15*time.Second)
	defer cancel()

	user, err := b.Store.User(reqCtx, ctx.AuthorID())
	if err != nil {
		return err
	}

	var setting *bool
	switch settingName {
	case "dm":
		setting = &user.DM
	case "crosspost":
		setting = &user.Crosspost
	case "ignore":
		setting = &user.Ignore
	default:
		return messages.ErrUnknownUserSetting(settingName)
	}

	old := *setting
	*setting = value

	_, err = b.Store.UpdateUser(reqCtx, user)
	if err != nil {
		return err
	}

	eb := embeds.NewBuilder()
	eb.InfoTemplate("Successfully changed user setting.")
	eb.AddField("Setting name", settingName, true)
	eb.AddField("Old setting", strconv.FormatBool(old), true)
	eb.AddField("New setting", strconv.FormatBool(value), true)

	return ctx.Reply(router.Embed(eb.Finalize()))
}

func unfav(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		var (
			id    int
			url   string
			err   error
			query = ctx.Options.String("query")
		)

		// If ID is not an integer assign query to the URL.
		if id, err = strconv.Atoi(query); err != nil {
			url = query
		}

		reqCtx, cancel := context.WithTimeout(ctx.Context(), 15*time.Second)
		defer cancel()

		var artwork *store.Artwork
		if url != "" {
			artwork, err = b.Store.Artwork(reqCtx, 0, url)
			if err != nil {
				return messages.ErrArtworkNotFound(query)
			}

			if artwork == nil {
				return messages.ErrArtworkNotFound(query)
			}

			id = artwork.ID
		}

		deleted, err := b.Store.DeleteBookmark(reqCtx, &store.Bookmark{UserID: ctx.AuthorID(), ArtworkID: id})
		if err != nil {
			return messages.ErrUserUnbookmarkFail(query, err)
		}

		if !deleted {
			return messages.ErrArtworkNotFound(query)
		}

		if artwork == nil {
			artwork, err = b.Store.Artwork(reqCtx, id, "")
			if err != nil || artwork == nil {
				eb := embeds.NewBuilder()
				locale := messages.BookmarkRemovedEmbed()

				eb.Title(locale.Title).
					Description(locale.Description).
					AddField("ID", strconv.Itoa(id), true)

				return ctx.Reply(router.Embed(eb.Finalize()))
			}
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

		return ctx.Reply(router.Embed(eb.Finalize()))
	}
}

func firstArtworkImage(artwork *store.Artwork) string {
	if artwork == nil || len(artwork.Images) == 0 {
		return ""
	}

	return artwork.Images[0]
}

func artworkToEmbed(artwork *store.Artwork, image string, ind, length int) *discordgo.MessageEmbed {
	title := ternary.If(
		length > 1,
		fmt.Sprintf(
			"[%v/%v] %v", ind+1, length,
			ternary.If(
				artwork.Title == "",
				artwork.Author,
				artwork.Title,
			),
		),
		fmt.Sprintf(
			"%v",
			ternary.If(
				artwork.Title == "",
				artwork.Author,
				artwork.Title,
			),
		),
	)

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

func initUser(b *bot.Bot, ctx *router.Context) (*store.User, error) {
	reqCtx, cancel := context.WithTimeout(ctx.Context(), 5*time.Second)
	defer cancel()

	return b.Store.User(reqCtx, ctx.AuthorID())
}

// handleStoreError returns an error if any store error is raised.
// If no error message is provided, handleStoreError will return the provided error or as nil.
func handleStoreError(err error, message ...error) error {
	if err == nil {
		return nil
	}

	switch {
	case errors.Is(err, store.ErrUserNotFound):
		if message != nil {
			return message[0]
		}

		return nil
	default:
		return err
	}
}

// successMessage builds and returns success message embed.
func successMessage(ctx *router.Context, message string) error {
	eb := embeds.NewBuilder()
	eb.SuccessTemplate(message)
	return ctx.Reply(router.Embed(eb.Finalize()))
}
