package commands

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/messages"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/mongo"
)

func GeneralGroup(b *bot.Bot) {
	group := "general"

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
		RateLimiter: gumi.NewRateLimiter(15 * time.Second),
		Exec:        set(b),
	})
}

func set(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		switch {
		case ctx.Args.Len() == 0:
			gd, err := ctx.Session.Guild(ctx.Event.GuildID)
			if err != nil {
				return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
			}

			guild, err := b.Guilds.FindOne(context.Background(), gd.ID)
			if err != nil {
				switch {
				case errors.Is(err, mongo.ErrNoDocuments):
					return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
				default:
					return err
				}
			}

			var (
				eb  = embeds.NewBuilder()
				msg = messages.Set()
			)

			eb.Title(msg.CurrentSettings).Description(fmt.Sprintf("**%v**", gd.Name))
			eb.Thumbnail(gd.IconURL())
			eb.Footer("Ebin message.", "")

			eb.AddField(
				msg.General.Title,
				fmt.Sprintf(
					"**%v**: %v | **%v**: %v",
					msg.General.Prefix, guild.Prefix,
					msg.General.NSFW, messages.FormatBool(guild.NSFW),
				),
			)

			eb.AddField(
				msg.Features.Title,
				fmt.Sprintf(
					"**%v**: %v | **%v**: %v | **%v**: %v",
					msg.Features.Repost, guild.Repost,
					msg.Features.Crosspost, messages.FormatBool(guild.Crosspost),
					msg.Features.Reactions, messages.FormatBool(guild.Reactions),
				),
			)

			eb.AddField(
				msg.PixivSettings.Title,
				fmt.Sprintf(
					"**%v**: %v | **%v**: %v",
					msg.PixivSettings.Enabled, messages.FormatBool(guild.Pixiv),
					msg.PixivSettings.Limit, strconv.Itoa(guild.Limit),
				),
			)

			eb.AddField(
				msg.TwitterSettings.Title,
				fmt.Sprintf(
					"**%v**: %v",
					msg.TwitterSettings.Enabled, messages.FormatBool(guild.Twitter),
				),
			)

			if len(guild.ArtChannels) > 0 {
				eb.AddField(
					msg.ArtChannels,
					strings.Join(arrays.MapString(guild.ArtChannels, func(s string) string {
						return fmt.Sprintf("<#%v> | `%v`", s, s)
					}), "\n"),
				)
			}

			ctx.ReplyEmbed(eb.Finalize())
			return nil
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

			guild, err := b.Guilds.FindOne(context.Background(), ctx.Event.GuildID)
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
			case "prefix":
				if unicode.IsLetter(rune(newSetting.Raw[len(newSetting.Raw)-1])) {
					newSetting.Raw += " "
				}

				if len(newSetting.Raw) > 5 {
					return messages.ErrPrefixTooLong(newSetting.Raw)
				}

				oldSettingEmbed = guild.Prefix
				newSettingEmbed = newSetting.Raw
				guild.Prefix = newSetting.Raw
			case "limit":
				limit, err := strconv.Atoi(newSetting.Raw)
				if err != nil {
					return messages.ErrParseInt(newSetting.Raw)
				}

				oldSettingEmbed = guild.Limit
				newSettingEmbed = limit
				guild.Limit = limit
			case "repost":
				if newSetting.Raw != "enabled" && newSetting.Raw != "disabled" && newSetting.Raw != "strict" {
					return messages.ErrUnknownRepostOption(newSetting.Raw)
				}

				oldSettingEmbed = guild.Repost
				newSettingEmbed = newSetting.Raw
				guild.Repost = newSetting.Raw
			case "nsfw":
				nsfw, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.NSFW
				newSettingEmbed = nsfw
				guild.NSFW = nsfw
			case "crosspost":
				crosspost, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.NSFW
				newSettingEmbed = crosspost
				guild.Crosspost = crosspost
			case "reactions":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.NSFW
				newSettingEmbed = new
				guild.Reactions = new
			case "pixiv":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.Pixiv
				newSettingEmbed = new
				guild.Pixiv = new
			case "twitter":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.Twitter
				newSettingEmbed = new
				guild.Twitter = new
			default:
				return messages.ErrUnknownSetting(settingName.Raw)
			}

			_, err = b.Guilds.ReplaceOne(context.Background(), guild)
			if err != nil {
				return err
			}

			eb := embeds.NewBuilder()
			eb.InfoTemplate("Successfully changed setting.")
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

func parseBool(s string) (bool, error) {
	s = strings.ToLower(s)
	if s == "true" || s == "enabled" || s == "on" {
		return true, nil
	}

	if s == "false" || s == "disabled" || s == "off" {
		return false, nil
	}

	return false, messages.ErrParseBool(s)
}
