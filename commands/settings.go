package commands

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"github.com/julien040/go-ternary"
	"go.mongodb.org/mongo-driver/mongo"
)

func settingsGroup(b *bot.Bot) {
	group := "settings"

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "set",
		Group:       group,
		Aliases:     []string{"cfg", "config", "settings"},
		Description: "Shows or edits server settings.",
		Usage:       "bt!set <setting name> <new setting>",
		Example:     "bt!set pixiv false",
		Flags:       make(map[string]string),
		GuildOnly:   true,
		NSFW:        false,
		AuthorOnly:  false,
		Permissions: 0,
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        set(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "artchannels",
		Group:       group,
		Aliases:     []string{"ac", "artchannel"},
		Description: "List or add/remove artchannels.",
		Usage:       "bt!artchannels <add/remove> [channel ids/category id...]",
		Example:     "bt!artchannels add #sfw #nsfw #basement",
		GuildOnly:   true,
		Permissions: discordgo.PermissionAdministrator | discordgo.PermissionManageServer,
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        artChannels(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "addchannel",
		Group:       group,
		Aliases:     []string{},
		Description: "Adds a new art channel to server settings.",
		Usage:       "bt!addchannel [channel ids/category id...]",
		Example:     "bt!addchannel #sfw #nsfw #basement",
		GuildOnly:   true,
		Permissions: discordgo.PermissionAdministrator | discordgo.PermissionManageServer,
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        addChannel(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "rmchannel",
		Group:       group,
		Aliases:     []string{"remchannel", "removechannel"},
		Description: "Removes an art channel from server settings.",
		Usage:       "bt!rmchannel [channel ids/category id...]",
		Example:     "bt!rmchannel #sfw #nsfw #basement",
		GuildOnly:   true,
		Permissions: discordgo.PermissionAdministrator | discordgo.PermissionManageServer,
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        removeChannel(b),
	})
}

func set(b *bot.Bot) func(*gumi.Ctx) error {
	return func(gctx *gumi.Ctx) error {
		showSettings := func() error {
			gd, err := gctx.Session.Guild(gctx.Event.GuildID)
			if err != nil {
				return messages.ErrGuildNotFound(err, gctx.Event.GuildID)
			}

			ctx, cancel := context.WithTimeout(b.Context, 5*time.Second)
			defer cancel()

			guild, err := b.Store.Guild(ctx, gd.ID)
			if err != nil {
				switch {
				case errors.Is(err, mongo.ErrNoDocuments):
					return messages.ErrGuildNotFound(err, gctx.Event.GuildID)
				default:
					return err
				}
			}

			eb := embeds.NewBuilder()
			eb.Title("Current settings").Description(fmt.Sprintf("**%v**", gd.Name))
			eb.Thumbnail(gd.IconURL("320"))
			eb.Footer("To change a setting use either its name or the name in parethesis", "")

			eb.AddField(
				"General",
				fmt.Sprintf(
					"**%v**: %v | **%v**: %v",
					"Prefix", guild.Prefix,
					"NSFW", messages.FormatBool(guild.NSFW),
				),
			)

			eb.AddField(
				"Features",
				fmt.Sprintf(
					"**%v**: %v | **%v**: %v\n**%v**: %v | **%v**: %v\n**%v**: %v | **%v**: %v",
					"Repost", guild.Repost,
					"Expiration (repost.expiration)", guild.RepostExpiration,
					"Crosspost", messages.FormatBool(guild.Crosspost),
					"Reactions", messages.FormatBool(guild.Reactions),
					"Tags", messages.FormatBool(guild.Tags),
					"Footer messages (footer)", messages.FormatBool(guild.FlavorText),
				),
			)

			eb.AddField(
				"Pixiv settings",
				fmt.Sprintf(
					"**%v**: %v | **%v**: %v",
					"Status (pixiv)", messages.FormatBool(guild.Pixiv),
					"Limit", strconv.Itoa(guild.Limit),
				),
			)

			eb.AddField(
				"Twitter settings",
				fmt.Sprintf(
					"**%v**: %v | **%v**: %v",
					"Status (twitter)", messages.FormatBool(guild.Twitter),
					"Skip First (twitter.skip)", messages.FormatBool(guild.SkipFirst),
				),
			)

			eb.AddField(
				"DeviantArt settings",
				fmt.Sprintf(
					"**%v**: %v",
					"Status (deviant)", messages.FormatBool(guild.Deviant),
				),
			)

			eb.AddField(
				"Bluesky settings",
				fmt.Sprintf(
					"**%v**: %v",
					"Status (bluesky)", messages.FormatBool(guild.Bluesky),
				),
			)

			channels := ternary.If(len(guild.ArtChannels) > 5,
				[]string{"There are more than 5 art channels, use `bt!artchannels` command to see them."},
				arrays.Map(guild.ArtChannels, func(s string) string {
					return fmt.Sprintf("<#%v> | `%v`", s, s)
				}),
			)

			eb.AddField(
				"Art channels",
				"Use `bt!artchannels` command to list or manage art channels!\n\n"+strings.Join(channels, "\n"),
			)

			return gctx.ReplyEmbed(eb.Finalize())
		}

		changeSetting := func() error {
			perms, err := dgoutils.MemberHasPermission(
				gctx.Session,
				gctx.Event.GuildID,
				gctx.Event.Author.ID,
				discordgo.PermissionAdministrator|discordgo.PermissionManageServer,
			)
			if err != nil {
				return err
			}

			if !perms {
				return gctx.Router.OnNoPermissionsCallback(gctx)
			}

			ctx, cancel := context.WithTimeout(b.Context, 10*time.Second)
			defer cancel()

			guild, err := b.Store.Guild(ctx, gctx.Event.GuildID)
			if err != nil {
				return err
			}

			var (
				settingName = gctx.Args.Get(0).Raw
				newSetting  = gctx.Args.Get(1).Raw
				oldSetting  any
			)

			switch settingName {
			case "prefix":
				if unicode.IsLetter(rune(newSetting[len(newSetting)-1])) {
					newSetting += " "
				}

				if len(newSetting) > 5 {
					return messages.ErrPrefixTooLong(newSetting)
				}

				oldSetting = guild.Prefix
				guild.Prefix = newSetting
			case "limit":
				limit, err := strconv.Atoi(newSetting)
				if err != nil {
					return messages.ErrParseInt(newSetting)
				}

				oldSetting = guild.Limit
				guild.Limit = limit
			case "repost":
				if newSetting != string(store.GuildRepostStrict) {
					enable, err := parseBool(newSetting)
					if err != nil {
						return messages.ErrUnknownRepostOption(newSetting)
					}

					newSetting = ternary.If(enable,
						string(store.GuildRepostEnabled),
						string(store.GuildRepostDisabled),
					)
				}

				oldSetting = guild.Repost
				guild.Repost = store.GuildRepost(newSetting)
			case "repost.expiration":
				dur, err := time.ParseDuration(newSetting)
				if err != nil {
					return messages.ErrParseDuration(newSetting)
				}

				if dur < 1*time.Minute || dur > 168*time.Hour {
					return messages.ErrExpirationOutOfRange(newSetting)
				}

				oldSetting = guild.RepostExpiration
				guild.RepostExpiration = dur
			case "nsfw", "crosspost", "reactions", "bluesky", "deviant", "deviantart", "pixiv", "twitter", "tags", "footer", "twitter.skip":
				enable, err := parseBool(newSetting)
				if err != nil {
					return err
				}

				switch settingName {
				case "nsfw":
					oldSetting = guild.NSFW
					guild.NSFW = enable
				case "crosspost":
					oldSetting = guild.Crosspost
					guild.Crosspost = enable
				case "reactions":
					oldSetting = guild.Reactions
					guild.Reactions = enable
				case "bluesky":
					oldSetting = guild.Bluesky
					guild.Bluesky = enable
				case "deviant", "deviantart":
					oldSetting = guild.Deviant
					guild.Deviant = enable
				case "pixiv":
					oldSetting = guild.Pixiv
					guild.Pixiv = enable
				case "twitter":
					oldSetting = guild.Twitter
					guild.Twitter = enable
				case "tags":
					oldSetting = guild.Tags
					guild.Tags = enable
				case "footer":
					oldSetting = guild.FlavorText
					guild.FlavorText = enable
				case "twitter.skip":
					oldSetting = guild.SkipFirst
					guild.SkipFirst = enable
				}
			default:
				return messages.ErrUnknownSetting(settingName)
			}

			_, err = b.Store.UpdateGuild(ctx, guild)
			if err != nil {
				return err
			}

			eb := embeds.NewBuilder()
			eb.InfoTemplate("Successfully changed setting.")
			eb.AddField("Setting name", settingName, true)
			eb.AddField("Old setting", fmt.Sprintf("%v", oldSetting), true)
			eb.AddField("New setting", fmt.Sprintf("%v", newSetting), true)

			return gctx.ReplyEmbed(eb.Finalize())
		}

		switch {
		case gctx.Args.Len() == 0:
			return showSettings()
		case gctx.Args.Len() >= 2:
			return changeSetting()
		default:
			return messages.ErrIncorrectCmd(gctx.Command)
		}
	}
}

func artChannels(b *bot.Bot) func(*gumi.Ctx) error {
	return func(gctx *gumi.Ctx) error {
		ctx, cancel := context.WithTimeout(b.Context, 10*time.Second)
		defer cancel()

		switch {
		case gctx.Args.Len() == 0:
			guild, err := b.Store.Guild(ctx, gctx.Event.GuildID)
			if err != nil {
				return messages.ErrGuildNotFound(err, gctx.Event.GuildID)
			}

			gd, err := gctx.Session.Guild(gctx.Event.GuildID)
			if err != nil {
				return messages.ErrGuildNotFound(err, gctx.Event.GuildID)
			}

			var (
				eb = embeds.NewBuilder()
				sb = &strings.Builder{}

				added int
			)

			eb.Title("Art channels")
			eb.Thumbnail(gd.IconURL("320"))
			if len(guild.ArtChannels) == 0 {
				eb.Description("You haven't added any art channels yet. Add your first art channel using `bt!artchannels add <channel mention>` command.")

				return gctx.ReplyEmbed(eb.Finalize())
			}

			eb.Footer("Total: "+strconv.Itoa(len(guild.ArtChannels)), "")
			channelEmbeds := make([]*discordgo.MessageEmbed, 0)
			for _, channel := range guild.ArtChannels {
				sb.WriteString(
					fmt.Sprintf("%v. <#%v> | `%v`\n", added+1, channel, channel),
				)

				added++
				if added%10 == 0 {
					eb.Description(sb.String())
					channelEmbeds = append(channelEmbeds, eb.Finalize())

					eb = embeds.NewBuilder()
					eb.Title("Art channels")
					eb.Thumbnail(gd.IconURL("320"))
					eb.Footer("Total: "+strconv.Itoa(len(guild.ArtChannels)), "")

					sb.Reset()
				}
			}

			if added%10 > 0 {
				eb.Description(sb.String())
				channelEmbeds = append(channelEmbeds, eb.Finalize())
			}

			wg := dgoutils.NewWidget(gctx.Session, gctx.Event.Author.ID, channelEmbeds)
			return wg.Start(gctx.Event.ChannelID)

		case gctx.Args.Len() >= 2:
			perms, err := dgoutils.MemberHasPermission(
				gctx.Session,
				gctx.Event.GuildID,
				gctx.Event.Author.ID,
				discordgo.PermissionAdministrator|discordgo.PermissionManageServer,
			)
			if err != nil {
				return err
			}

			if !perms {
				return gctx.Router.OnNoPermissionsCallback(gctx)
			}

			var (
				action = gctx.Args.Get(0)

				filter  func(guild *store.Guild, channelID string) error
				execute func(guildID string, channels []string) error
			)

			switch action.Raw {
			case "add":
				execute = func(guildID string, channels []string) error {
					if _, err := b.Store.AddArtChannels(ctx, guildID, channels); err != nil {
						return err
					}

					eb := embeds.NewBuilder()
					eb.SuccessTemplate(messages.AddArtChannelSuccess(channels))
					return gctx.ReplyEmbed(eb.Finalize())
				}

				filter = func(guild *store.Guild, channelID string) error {
					exists := false
					for _, artChannelID := range guild.ArtChannels {
						if artChannelID == channelID {
							exists = true
						}
					}

					if exists {
						return messages.ErrAlreadyArtChannel(channelID)
					}

					return nil
				}
			case "remove":
				execute = func(guildID string, channels []string) error {
					if _, err := b.Store.DeleteArtChannels(ctx, guildID, channels); err != nil {
						return err
					}

					eb := embeds.NewBuilder()
					eb.SuccessTemplate(messages.RemoveArtChannelSuccess(channels))
					return gctx.ReplyEmbed(eb.Finalize())
				}

				filter = func(guild *store.Guild, channelID string) error {
					exists := false
					for _, artChannelID := range guild.ArtChannels {
						if artChannelID == channelID {
							exists = true
						}
					}

					if !exists {
						return messages.ErrNotArtChannel(channelID)
					}

					return nil
				}
			}

			guild, err := b.Store.Guild(ctx, gctx.Event.GuildID)
			if err != nil {
				return messages.ErrGuildNotFound(err, gctx.Event.GuildID)
			}

			channels := make([]string, 0)
			for _, arg := range gctx.Args.Arguments[1:] {
				ch, err := gctx.Session.Channel(dgoutils.TrimmerRaw(arg.Raw))
				if err != nil {
					return err
				}

				if ch.GuildID != guild.ID {
					return messages.ErrForeignChannel(ch.ID)
				}

				if ch.Type == discordgo.ChannelTypeGuildVoice {
					continue
				}

				switch ch.Type {
				case discordgo.ChannelTypeGuildCategory:
					gcs, err := gctx.Session.GuildChannels(guild.ID)
					if err != nil {
						return err
					}

					for _, gc := range gcs {
						if ch.Type == discordgo.ChannelTypeGuildVoice {
							continue
						}

						if gc.ParentID == ch.ID {
							if err := filter(guild, ch.ID); err != nil {
								return err
							}

							channels = append(channels, gc.ID)
						}
					}
				default:
					if err := filter(guild, ch.ID); err != nil {
						return err
					}

					channels = append(channels, ch.ID)
				}
			}

			return execute(guild.ID, channels)
		default:
			return messages.ErrIncorrectCmd(gctx.Command)
		}
	}
}

func addChannel(b *bot.Bot) func(*gumi.Ctx) error {
	return func(gctx *gumi.Ctx) error {
		if err := dgoutils.ValidateArgs(gctx, 1); err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(b.Context, 10*time.Second)
		defer cancel()

		guild, err := b.Store.Guild(ctx, gctx.Event.GuildID)
		if err != nil {
			return messages.ErrGuildNotFound(err, gctx.Event.GuildID)
		}

		channels := make([]string, 0)
		for _, arg := range gctx.Args.Arguments {
			ch, err := gctx.Session.Channel(dgoutils.TrimmerRaw(arg.Raw))
			if err != nil {
				return err
			}

			if ch.GuildID != guild.ID {
				return messages.ErrForeignChannel(ch.ID)
			}

			switch ch.Type {
			case discordgo.ChannelTypeGuildText:
				exists := false
				for _, channelID := range guild.ArtChannels {
					if channelID == ch.ID {
						exists = true
					}
				}

				if exists {
					return messages.ErrAlreadyArtChannel(ch.ID)
				}

				channels = append(channels, ch.ID)
			case discordgo.ChannelTypeGuildCategory:
				gcs, err := gctx.Session.GuildChannels(guild.ID)
				if err != nil {
					return err
				}

				for _, gc := range gcs {
					if gc.Type != discordgo.ChannelTypeGuildText {
						continue
					}

					if gc.ParentID == ch.ID {
						exists := false
						for _, channelID := range guild.ArtChannels {
							if channelID == gc.ID {
								exists = true
							}
						}

						if exists {
							return messages.ErrAlreadyArtChannel(ch.ID)
						}

						channels = append(channels, gc.ID)
					}
				}
			default:
				return nil
			}
		}

		_, err = b.Store.AddArtChannels(
			ctx,
			guild.ID,
			channels,
		)
		if err != nil {
			return err
		}

		eb := embeds.NewBuilder()
		eb.SuccessTemplate(messages.AddArtChannelSuccess(channels))
		return gctx.ReplyEmbed(eb.Finalize())
	}
}

func removeChannel(b *bot.Bot) func(*gumi.Ctx) error {
	return func(gctx *gumi.Ctx) error {
		if err := dgoutils.ValidateArgs(gctx, 1); err != nil {
			return err
		}

		ctx, cancel := context.WithTimeout(b.Context, 10*time.Second)
		defer cancel()

		guild, err := b.Store.Guild(ctx, gctx.Event.GuildID)
		if err != nil {
			return messages.ErrGuildNotFound(err, gctx.Event.GuildID)
		}

		channels := make([]string, 0)
		for _, arg := range gctx.Args.Arguments {
			ch, err := gctx.Session.Channel(dgoutils.TrimmerRaw(arg.Raw))
			if err != nil {
				if !strings.Contains(err.Error(), "404") {
					return messages.ErrChannelNotFound(err, arg.Raw)
				}

				channels = append(channels, dgoutils.TrimmerRaw(arg.Raw))
				continue
			}

			if ch.GuildID != gctx.Event.GuildID {
				return messages.ErrForeignChannel(ch.ID)
			}

			switch ch.Type {
			case discordgo.ChannelTypeGuildText:
				channels = append(channels, ch.ID)
			case discordgo.ChannelTypeGuildCategory:
				gcs, err := gctx.Session.GuildChannels(guild.ID)
				if err != nil {
					return err
				}

				for _, gc := range gcs {
					if gc.Type != discordgo.ChannelTypeGuildText {
						continue
					}

					if gc.ParentID == ch.ID {
						channels = append(channels, gc.ID)
					}
				}
			default:
				return nil
			}
		}

		_, err = b.Store.DeleteArtChannels(
			ctx,
			guild.ID,
			channels,
		)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return messages.RemoveArtChannelFail(channels)
			}

			return err
		}

		eb := embeds.NewBuilder()
		eb.SuccessTemplate(messages.RemoveArtChannelSuccess(channels))
		return gctx.ReplyEmbed(eb.Finalize())
	}
}

func parseBool(s string) (bool, error) {
	s = strings.ToLower(s)
	if slices.Contains([]string{"enable", "enabled", "true", "on"}, s) {
		return true, nil
	}

	if slices.Contains([]string{"disable", "disabled", "false", "off"}, s) {
		return false, nil
	}

	return false, messages.ErrParseBool(s)
}
