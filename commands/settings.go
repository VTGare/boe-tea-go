package commands

import (
	"context"
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
	"github.com/VTGare/boe-tea-go/router"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/embeds"
	"github.com/bwmarrin/discordgo"
	"github.com/julien040/go-ternary"
)

var guildManagePerms int64 = discordgo.PermissionAdministrator | discordgo.PermissionManageGuild

func settingsGroup(b *bot.Bot) []*router.Command {
	settingChoices := []router.Choice{
		{Name: "prefix", Value: "prefix"},
		{Name: "limit", Value: "limit"},
		{Name: "repost", Value: "repost"},
		{Name: "repost.expiration", Value: "repost.expiration"},
		{Name: "nsfw", Value: "nsfw"},
		{Name: "crosspost", Value: "crosspost"},
		{Name: "reactions", Value: "reactions"},
		{Name: "pixiv", Value: "pixiv"},
		{Name: "bluesky", Value: "bluesky"},
		{Name: "twitter", Value: "twitter"},
		{Name: "deviant", Value: "deviant"},
		{Name: "tags", Value: "tags"},
		{Name: "footer", Value: "footer"},
		{Name: "twitter.skip", Value: "twitter.skip"},
	}

	manageChecks := []router.Check{router.GuildOnly, router.HasPermissions(guildManagePerms)}

	return []*router.Command{
		{
			Name:        "set",
			Category:    "Settings",
			Aliases:     []string{"cfg", "config", "settings"},
			Description: "Shows or edits server settings.",
			Checks:      []router.Check{router.GuildOnly},
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 5*time.Second),
			Options: []*router.Option{
				router.String("setting", "Setting to change").WithChoices(settingChoices...),
				router.String("value", "New value").Greedy(),
			},
			Examples: []string{"set pixiv false"},
			Handler:  set(b),
		},
		{
			Name:        "artchannels",
			Category:    "Settings",
			Aliases:     []string{"ac", "artchannel"},
			Description: "List or add/remove artchannels.",
			Checks:      manageChecks,
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 5*time.Second),
			Options: []*router.Option{
				router.String("action", "What to do").WithChoices(
					router.Choice{Name: "add", Value: "add"},
					router.Choice{Name: "remove", Value: "remove"},
				),
				router.String("targets", "Channels or categories").Greedy(),
			},
			Examples: []string{"artchannels add #sfw #nsfw #basement"},
			Handler:  artChannels(b),
		},
		{
			Name:        "addchannel",
			Category:    "Settings",
			Description: "Adds a new art channel to server settings.",
			Checks:      manageChecks,
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 5*time.Second),
			Options: []*router.Option{
				router.String("targets", "Channel IDs, mentions, or categories").Require().Greedy(),
			},
			Examples: []string{"addchannel #sfw #nsfw #basement"},
			Handler:  addChannel(b),
		},
		{
			Name:        "rmchannel",
			Category:    "Settings",
			Aliases:     []string{"remchannel", "removechannel"},
			Description: "Removes an art channel from server settings.",
			Checks:      manageChecks,
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 5*time.Second),
			Options: []*router.Option{
				router.String("targets", "Channel IDs, mentions, or categories").Require().Greedy(),
			},
			Examples: []string{"rmchannel #sfw #nsfw #basement"},
			Handler:  removeChannel(b),
		},
	}
}

func set(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		setting := ctx.Options.String("setting")
		if setting == "" {
			return showSettings(b, ctx)
		}

		value := ctx.Options.String("value")
		if value == "" {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		return changeSetting(b, ctx, setting, value)
	}
}

func showSettings(b *bot.Bot, ctx *router.Context) error {
	gd, err := ctx.Session.Guild(ctx.GuildID())
	if err != nil {
		return messages.ErrGuildNotFound(err, ctx.GuildID())
	}

	reqCtx, cancel := context.WithTimeout(ctx.Context(), 5*time.Second)
	defer cancel()

	guild, err := b.Store.Guild(reqCtx, gd.ID)
	if err != nil {
		switch {
		case errors.Is(err, store.ErrGuildNotFound):
			return messages.ErrGuildNotFound(err, ctx.GuildID())
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

	channels := ternary.If(
		len(guild.ArtChannels) > 5,
		[]string{"There are more than 5 art channels, use `bt!artchannels` command to see them."},
		arrays.Map(guild.ArtChannels, func(s string) string {
			return fmt.Sprintf("<#%v> | `%v`", s, s)
		}),
	)

	eb.AddField(
		"Art channels",
		"Use `bt!artchannels` command to list or manage art channels!\n\n"+strings.Join(channels, "\n"),
	)

	return ctx.Reply(router.Embed(eb.Finalize()))
}

func changeSetting(b *bot.Bot, ctx *router.Context, settingName, newSetting string) error {
	if err := router.HasPermissions(guildManagePerms)(ctx); err != nil {
		return err
	}

	reqCtx, cancel := context.WithTimeout(ctx.Context(), 10*time.Second)
	defer cancel()

	guild, err := b.Store.Guild(reqCtx, ctx.GuildID())
	if err != nil {
		return err
	}

	var (
		newSettingEmbed any
		oldSettingEmbed any
	)

	applySetting := func(guildSet any, newSet any) any {
		oldSettingEmbed = guildSet
		newSettingEmbed = newSet
		return newSet
	}

	switch settingName {
	case "prefix":
		if len(newSetting) > 0 && unicode.IsLetter(rune(newSetting[len(newSetting)-1])) {
			newSetting += " "
		}

		if len(newSetting) > 5 {
			return messages.ErrPrefixTooLong(newSetting)
		}

		applySetting(guild.Prefix, newSetting)
		guild.Prefix = newSetting
	case "limit":
		limit, err := strconv.Atoi(newSetting)
		if err != nil {
			return messages.ErrParseInt(newSetting)
		}

		applySetting(guild.Limit, limit)
		guild.Limit = limit
	case "repost":
		if newSetting != string(store.GuildRepostEnabled) &&
			newSetting != string(store.GuildRepostDisabled) &&
			newSetting != string(store.GuildRepostStrict) {
			return messages.ErrUnknownRepostOption(newSetting)
		}

		applySetting(guild.Repost, newSetting)
		guild.Repost = store.GuildRepost(newSetting)

	case "repost.expiration":
		dur, err := time.ParseDuration(newSetting)
		if err != nil {
			return messages.ErrParseDuration(newSetting)
		}

		if dur < 1*time.Minute || dur > 168*time.Hour {
			return messages.ErrExpirationOutOfRange(newSetting)
		}

		applySetting(guild.RepostExpiration, dur)
		guild.RepostExpiration = dur

	case "nsfw":
		enable, err := parseBool(newSetting)
		if err != nil {
			return err
		}

		applySetting(guild.NSFW, enable)
		guild.NSFW = enable

	case "crosspost":
		enable, err := parseBool(newSetting)
		if err != nil {
			return err
		}

		applySetting(guild.Crosspost, enable)
		guild.Crosspost = enable

	case "reactions":
		enable, err := parseBool(newSetting)
		if err != nil {
			return err
		}

		applySetting(guild.Reactions, enable)
		guild.Reactions = enable

	case "pixiv":
		enable, err := parseBool(newSetting)
		if err != nil {
			return err
		}

		applySetting(guild.Pixiv, enable)
		guild.Pixiv = enable

	case "bluesky":
		enable, err := parseBool(newSetting)
		if err != nil {
			return err
		}

		applySetting(guild.Bluesky, enable)
		guild.Bluesky = enable

	case "twitter":
		enable, err := parseBool(newSetting)
		if err != nil {
			return err
		}

		applySetting(guild.Twitter, enable)
		guild.Twitter = enable

	case "deviant":
		enable, err := parseBool(newSetting)
		if err != nil {
			return err
		}

		applySetting(guild.Deviant, enable)
		guild.Deviant = enable

	case "tags":
		enable, err := parseBool(newSetting)
		if err != nil {
			return err
		}

		applySetting(guild.Tags, enable)
		guild.Tags = enable

	case "footer":
		enable, err := parseBool(newSetting)
		if err != nil {
			return err
		}

		applySetting(guild.FlavorText, enable)
		guild.FlavorText = enable

	case "twitter.skip":
		enable, err := parseBool(newSetting)
		if err != nil {
			return err
		}

		applySetting(guild.SkipFirst, enable)
		guild.SkipFirst = enable

	default:
		return messages.ErrUnknownSetting(settingName)
	}

	_, err = b.Store.UpdateGuild(reqCtx, guild)
	if err != nil {
		return err
	}

	eb := embeds.NewBuilder()
	eb.InfoTemplate("Successfully changed setting.")
	eb.AddField("Setting name", settingName, true)
	eb.AddField("Old setting", fmt.Sprintf("%v", oldSettingEmbed), true)
	eb.AddField("New setting", fmt.Sprintf("%v", newSettingEmbed), true)

	return ctx.Reply(router.Embed(eb.Finalize()))
}

func artChannels(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		reqCtx, cancel := context.WithTimeout(ctx.Context(), 10*time.Second)
		defer cancel()

		action := ctx.Options.String("action")
		targets := ctx.Options.String("targets")

		if action == "" {
			return listArtChannels(b, ctx)
		}

		if targets == "" {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		var execute func(guildID string, channels []string) error

		switch action {
		case "add":
			execute = func(guildID string, channels []string) error {
				if _, err := b.Store.AddArtChannels(reqCtx, guildID, channels); err != nil {
					return err
				}

				eb := embeds.NewBuilder()
				eb.SuccessTemplate(messages.AddArtChannelSuccess(channels))
				return ctx.Reply(router.Embed(eb.Finalize()))
			}
		case "remove":
			execute = func(guildID string, channels []string) error {
				if _, err := b.Store.DeleteArtChannels(reqCtx, guildID, channels); err != nil {
					return err
				}

				eb := embeds.NewBuilder()
				eb.SuccessTemplate(messages.RemoveArtChannelSuccess(channels))
				return ctx.Reply(router.Embed(eb.Finalize()))
			}
		default:
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		guild, err := b.Store.Guild(reqCtx, ctx.GuildID())
		if err != nil {
			return messages.ErrGuildNotFound(err, ctx.GuildID())
		}

		channels := make([]string, 0)
		for _, arg := range strings.Fields(targets) {
			ch, err := ctx.Session.Channel(dgoutils.TrimmerRaw(arg))
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
					if gc.Type == discordgo.ChannelTypeGuildVoice {
						continue
					}

					if gc.ParentID == ch.ID {
						if err := checkArtChannel(guild, gc.ID, action); err != nil {
							return err
						}

						channels = append(channels, gc.ID)
					}
				}
			default:
				if err := checkArtChannel(guild, ch.ID, action); err != nil {
					return err
				}

				channels = append(channels, ch.ID)
			}
		}

		return execute(guild.ID, channels)
	}
}

func checkArtChannel(guild *store.Guild, channelID, action string) error {
	exists := false
	for _, artChannelID := range guild.ArtChannels {
		if artChannelID == channelID {
			exists = true
		}
	}

	switch action {
	case "add":
		if exists {
			return messages.ErrAlreadyArtChannel(channelID)
		}
	case "remove":
		if !exists {
			return messages.ErrNotArtChannel(channelID)
		}
	}

	return nil
}

func listArtChannels(b *bot.Bot, ctx *router.Context) error {
	reqCtx, cancel := context.WithTimeout(ctx.Context(), 10*time.Second)
	defer cancel()

	guild, err := b.Store.Guild(reqCtx, ctx.GuildID())
	if err != nil {
		return messages.ErrGuildNotFound(err, ctx.GuildID())
	}

	gd, err := ctx.Session.Guild(ctx.GuildID())
	if err != nil {
		return messages.ErrGuildNotFound(err, ctx.GuildID())
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

		return ctx.Reply(router.Embed(eb.Finalize()))
	}

	eb.Footer("Total: "+strconv.Itoa(len(guild.ArtChannels)), "")
	channelEmbeds := make([]*discordgo.MessageEmbed, 0)
	for _, channel := range guild.ArtChannels {
		fmt.Fprintf(sb, "%v. <#%v> | `%v`\n", added+1, channel, channel)

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

	return replyPages(ctx, b, channelEmbeds)
}

func addChannel(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		reqCtx, cancel := context.WithTimeout(ctx.Context(), 10*time.Second)
		defer cancel()

		guild, err := b.Store.Guild(reqCtx, ctx.GuildID())
		if err != nil {
			return messages.ErrGuildNotFound(err, ctx.GuildID())
		}

		channels := make([]string, 0)
		for _, arg := range strings.Fields(ctx.Options.String("targets")) {
			ch, err := ctx.Session.Channel(dgoutils.TrimmerRaw(arg))
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
			reqCtx,
			guild.ID,
			channels,
		)
		if err != nil {
			return err
		}

		eb := embeds.NewBuilder()
		eb.SuccessTemplate(messages.AddArtChannelSuccess(channels))
		return ctx.Reply(router.Embed(eb.Finalize()))
	}
}

func removeChannel(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		reqCtx, cancel := context.WithTimeout(ctx.Context(), 10*time.Second)
		defer cancel()

		guild, err := b.Store.Guild(reqCtx, ctx.GuildID())
		if err != nil {
			return messages.ErrGuildNotFound(err, ctx.GuildID())
		}

		channels := make([]string, 0)
		for _, arg := range strings.Fields(ctx.Options.String("targets")) {
			ch, err := ctx.Session.Channel(dgoutils.TrimmerRaw(arg))
			if err != nil {
				if !strings.Contains(err.Error(), "404") {
					return messages.ErrChannelNotFound(err, arg)
				}

				channels = append(channels, dgoutils.TrimmerRaw(arg))
				continue
			}

			if ch.GuildID != ctx.GuildID() {
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
			reqCtx,
			guild.ID,
			channels,
		)
		if err != nil {
			if errors.Is(err, store.ErrGuildNotFound) {
				return messages.RemoveArtChannelFail(channels)
			}

			return err
		}

		eb := embeds.NewBuilder()
		eb.SuccessTemplate(messages.RemoveArtChannelSuccess(channels))
		return ctx.Reply(router.Embed(eb.Finalize()))
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
