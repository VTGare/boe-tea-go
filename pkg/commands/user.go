package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/messages"
	"github.com/VTGare/boe-tea-go/pkg/models/users"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/mongo"
)

func UserGroup(b *bot.Bot) {
	group := "user"

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "profile",
		Group:       group,
		Description: "Shows user's profile and settings.",
		Usage:       "bt!profile",
		Example:     "bt!profile",
		RateLimiter: gumi.NewRateLimiter(15 * time.Second),
		Exec:        profile(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "groups",
		Group:       group,
		Aliases:     []string{"ls"},
		Description: "Shows all crosspost groups.",
		Usage:       "bt!groups",
		Example:     "bt!groups",
		RateLimiter: gumi.NewRateLimiter(15 * time.Second),
		Exec:        groups(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "newgroup",
		Group:       group,
		Aliases:     []string{"addgroup"},
		Description: "Creates a new crosspost group.",
		Usage:       "bt!newgroup <group name> <parent channel>",
		Example:     "bt!newgroup lewds #nsfw",
		RateLimiter: gumi.NewRateLimiter(15 * time.Second),
		Exec:        newgroup(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "delgroup",
		Group:       group,
		Description: "Deletes a crosspost group.",
		Usage:       "bt!delgroup <group name>",
		Example:     "bt!delgroup schooldays",
		RateLimiter: gumi.NewRateLimiter(15 * time.Second),
		Exec:        delgroup(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "push",
		Group:       group,
		Aliases:     []string{},
		Description: "Adds channels to a crosspost group.",
		Usage:       "bt!push <group name> [channel ids]",
		Example:     "bt!push myCoolGroup #coolchannel #coolerchannel",
		RateLimiter: gumi.NewRateLimiter(15 * time.Second),
		Exec:        push(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "remove",
		Group:       group,
		Aliases:     []string{"pop"},
		Description: "Removes channels from a crosspost group",
		Usage:       "bt!remove <group name> [channel ids]",
		Example:     "bt!remove cuteAnimeGirls #nsfw-channel #cat-pics",
		RateLimiter: gumi.NewRateLimiter(15 * time.Second),
		Exec:        remove(b),
	})
}

func findOrCreateUser(b *bot.Bot, userID string) (*users.User, error) {
	user, err := b.Users.FindOne(context.Background(), userID)
	if err != nil {
		switch {
		case errors.Is(err, mongo.ErrNoDocuments):
			user, err = b.Users.InsertOne(
				context.Background(),
				userID,
			)

			if err != nil {
				return nil, err
			}
		default:
			return nil, messages.ErrUserNotFound(err, userID)
		}
	}

	return user, nil
}

func profile(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		user, err := findOrCreateUser(b, ctx.Event.Author.ID)
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
	}
}

func groups(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		user, err := findOrCreateUser(b, ctx.Event.Author.ID)
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

		user, err := findOrCreateUser(b, ctx.Event.Author.ID)
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

		user, err := findOrCreateUser(b, ctx.Event.Author.ID)
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

		user, err := findOrCreateUser(b, ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		name := ctx.Args.Get(0).Raw
		ctx.Args.Remove(0)

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

			if group, ok := user.FindGroup(channelID); ok {
				if group.Name == name {
					continue
				}
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
			return messages.UserPushFail(name)
		}

		return nil
	}
}

func remove(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() < 2 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		user, err := findOrCreateUser(b, ctx.Event.Author.ID)
		if err != nil {
			return err
		}

		name := ctx.Args.Get(0).Raw
		ctx.Args.Remove(0)

		removed := make([]string, 0, ctx.Args.Len())
		for _, arg := range ctx.Args.Arguments {
			channelID := strings.Trim(arg.Raw, "<#>")

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
			return messages.UserRemoveFail(name)
		}

		return nil
	}
}
