package commands

import (
	"errors"
	"fmt"
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
		Flags:       map[string]string{},
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

func set(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		showSettings := func() error {
			gd, err := ctx.Session.Guild(ctx.Event.GuildID)
			if err != nil {
				return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
			}

			guild, err := b.Store.Guild(b.Context, gd.ID)
			if err != nil {
				switch {
				case errors.Is(err, mongo.ErrNoDocuments):
					return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
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
					"CrossPost", messages.FormatBool(guild.Crosspost),
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
				"ArtStation settings",
				fmt.Sprintf(
					"**%v**: %v",
					"Status (artstation)", messages.FormatBool(guild.Artstation),
				),
			)

			var artChannels []string
			if len(guild.ArtChannels) > 5 {
				artChannels = []string{"There are more than 5 art channels, use `bt!artchannels` command to see them."}
			} else {
				artChannels = arrays.Map(guild.ArtChannels, func(s string) string {
					return fmt.Sprintf("<#%v> | `%v`", s, s)
				})
			}

			eb.AddField(
				"Art channels",
				"Use `bt!artchannels` command to list or manage art channels!\n\n"+strings.Join(artChannels, "\n"),
			)

			return ctx.ReplyEmbed(eb.Finalize())
		}

		changeSetting := func() error {
			perms, err := dgoutils.MemberHasPermission(
				ctx.Session,
				ctx.Event.GuildID,
				ctx.Event.Author.ID,
				discordgo.PermissionAdministrator|discordgo.PermissionManageServer,
			)
			if err != nil {
				return err
			}

			if !perms {
				return ctx.Router.OnNoPermissionsCallback(ctx)
			}

			guild, err := b.Store.Guild(b.Context, ctx.Event.GuildID)
			if err != nil {
				return err
			}

			var (
				settingName     = ctx.Args.Get(0)
				newSetting      = ctx.Args.Get(1)
				newSettingEmbed any
				oldSettingEmbed any
			)

			applySetting := func(guildSet any, newSet any) any {
				oldSettingEmbed = guildSet
				newSettingEmbed = newSet
				return newSet
			}

			switch settingName.Raw {
			case "prefix":
				if unicode.IsLetter(rune(newSetting.Raw[len(newSetting.Raw)-1])) {
					newSetting.Raw += " "
				}

				if len(newSetting.Raw) > 5 {
					return messages.ErrPrefixTooLong(newSetting.Raw)
				}

				guild.Prefix = applySetting(guild.Prefix, newSetting.Raw).(string)
			case "limit":
				limit, err := strconv.Atoi(newSetting.Raw)
				if err != nil {
					return messages.ErrParseInt(newSetting.Raw)
				}

				guild.Limit = applySetting(guild.Limit, limit).(int)
			case "repost":
				if newSetting.Raw != string(store.GuildRepostEnabled) &&
					newSetting.Raw != string(store.GuildRepostDisabled) &&
					newSetting.Raw != string(store.GuildRepostStrict) {
					return messages.ErrUnknownRepostOption(newSetting.Raw)
				}

				guild.Repost = store.GuildRepost(applySetting(guild.Repost, newSetting.Raw).(string))

				//guild.Repost = store.GuildRepost(newSetting.Raw)
			case "repost.expiration":
				dur, err := time.ParseDuration(newSetting.Raw)
				if err != nil {
					return messages.ErrParseDuration(newSetting.Raw)
				}

				if dur < 1*time.Minute || dur > 168*time.Hour {
					return messages.ErrExpirationOutOfRange(newSetting.Raw)
				}

				guild.RepostExpiration = applySetting(guild.RepostExpiration, dur).(time.Duration)
			case "nsfw":
				enable, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				applySetting(guild.NSFW, enable)
			case "crosspost":
				enable, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				guild.Crosspost = applySetting(guild.Crosspost, enable).(bool)
			case "reactions":
				enable, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				applySetting(guild.Reactions, enable)
			case "pixiv":
				enable, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				guild.Pixiv = applySetting(guild.Pixiv, enable).(bool)
			case "twitter":
				enable, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				guild.Twitter = applySetting(guild.Twitter, enable).(bool)
			case "deviant":
				enable, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				guild.Deviant = applySetting(guild.Deviant, enable).(bool)
			case "artstation":
				enable, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				applySetting(guild.Artstation, enable)
			case "tags":
				enable, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				guild.Tags = applySetting(guild.Tags, enable).(bool)
			case "footer":
				enable, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				guild.FlavorText = applySetting(guild.FlavorText, enable).(bool)
			case "twitter.skip":
				enable, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				guild.SkipFirst = applySetting(guild.SkipFirst, enable).(bool)
			default:
				return messages.ErrUnknownSetting(settingName.Raw)
			}

			_, err = b.Store.UpdateGuild(b.Context, guild)
			if err != nil {
				return err
			}

			eb := embeds.NewBuilder()
			eb.InfoTemplate("Successfully changed setting.")
			eb.AddField("Setting name", settingName.Raw, true)
			eb.AddField("Old setting", fmt.Sprintf("%v", oldSettingEmbed), true)
			eb.AddField("New setting", fmt.Sprintf("%v", newSettingEmbed), true)

			return ctx.ReplyEmbed(eb.Finalize())
		}

		switch {
		case ctx.Args.Len() == 0:
			return showSettings()
		case ctx.Args.Len() >= 2:
			return changeSetting()
		default:
			return messages.ErrIncorrectCmd(ctx.Command)
		}
	}
}

func artChannels(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		switch {
		case ctx.Args.Len() == 0:
			guild, err := b.Store.Guild(b.Context, ctx.Event.GuildID)
			if err != nil {
				return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
			}

			gd, err := ctx.Session.Guild(ctx.Event.GuildID)
			if err != nil {
				return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
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

				return ctx.ReplyEmbed(eb.Finalize())
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

			wg := dgoutils.NewWidget(ctx.Session, ctx.Event.Author.ID, channelEmbeds)
			return wg.Start(ctx.Event.ChannelID)

		case ctx.Args.Len() >= 2:
			perms, err := dgoutils.MemberHasPermission(
				ctx.Session,
				ctx.Event.GuildID,
				ctx.Event.Author.ID,
				discordgo.PermissionAdministrator|discordgo.PermissionManageServer,
			)
			if err != nil {
				return err
			}

			if !perms {
				return ctx.Router.OnNoPermissionsCallback(ctx)
			}

			var (
				action = ctx.Args.Get(0)

				filter  func(guild *store.Guild, channelID string) error
				execute func(guildID string, channels []string) error
			)

			switch action.Raw {
			case "add":
				execute = func(guildID string, channels []string) error {
					if _, err := b.Store.AddArtChannels(b.Context, guildID, channels); err != nil {
						return err
					}

					eb := embeds.NewBuilder()
					eb.SuccessTemplate(messages.AddArtChannelSuccess(channels))
					return ctx.ReplyEmbed(eb.Finalize())
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
					if _, err := b.Store.DeleteArtChannels(b.Context, guildID, channels); err != nil {
						return err
					}

					eb := embeds.NewBuilder()
					eb.SuccessTemplate(messages.RemoveArtChannelSuccess(channels))
					return ctx.ReplyEmbed(eb.Finalize())
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

			guild, err := b.Store.Guild(b.Context, ctx.Event.GuildID)
			if err != nil {
				return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
			}

			channels := make([]string, 0)
			for arg := range ctx.Args.Arguments[1:] {
				ch, err := ctx.Session.Channel(dgoutils.Trimmer(ctx, arg))
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
					gcs, err := ctx.Session.GuildChannels(guild.ID)
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
			return messages.ErrIncorrectCmd(ctx.Command)
		}
	}
}

func addChannel(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if err := dgoutils.InitCommand(ctx, 1); err != nil {
			return err
		}

		guild, err := b.Store.Guild(b.Context, ctx.Event.GuildID)
		if err != nil {
			return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
		}

		channels := make([]string, 0)
		for _, arg := range ctx.Args.Arguments {
			ch, err := ctx.Session.Channel(strings.Trim(arg.Raw, "<#>"))
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
				gcs, err := ctx.Session.GuildChannels(guild.ID)
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
			b.Context,
			guild.ID,
			channels,
		)
		if err != nil {
			return err
		}

		eb := embeds.NewBuilder()
		eb.SuccessTemplate(messages.AddArtChannelSuccess(channels))
		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func removeChannel(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if err := dgoutils.InitCommand(ctx, 1); err != nil {
			return err
		}

		guild, err := b.Store.Guild(b.Context, ctx.Event.GuildID)
		if err != nil {
			return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
		}

		channels := make([]string, 0)
		for _, arg := range ctx.Args.Arguments {
			ch, err := ctx.Session.Channel(strings.Trim(arg.Raw, "<#>"))
			if err != nil {
				return messages.ErrChannelNotFound(err, arg.Raw)
			}

			if ch.GuildID != ctx.Event.GuildID {
				return messages.ErrForeignChannel(ch.ID)
			}

			switch ch.Type {
			case discordgo.ChannelTypeGuildText:
				channels = append(channels, ch.ID)
			case discordgo.ChannelTypeGuildCategory:
				gcs, err := ctx.Session.GuildChannels(guild.ID)
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
			b.Context,
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
		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func parseBool(s string) (bool, error) {
	s = strings.ToLower(s)
	if s == "true" || s == "enable" || s == "enabled" || s == "on" {
		return true, nil
	}

	if s == "false" || s == "disable" || s == "disabled" || s == "off" {
		return false, nil
	}

	return false, messages.ErrParseBool(s)
}
