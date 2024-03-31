package commands

import (
	"context"
	"errors"
	"fmt"
	"runtime"
	"sort"
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

func generalGroup(b *bot.Bot) {
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
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        set(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "about",
		Group:       group,
		Aliases:     []string{"invite", "patreon", "support"},
		Description: "Bot's about page with the invite link and other useful stuff.",
		Usage:       "bt!about",
		Example:     "bt!about",
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        about(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "help",
		Group:       group,
		Aliases:     []string{"documentation", "docs"},
		Description: "Shows this page.",
		Usage:       "bt!help <group/command name>",
		Example:     "bt!help",
		Exec:        help(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "ping",
		Group:       group,
		Description: "Checks bot's availabity and response time.",
		Usage:       "bt!ping",
		Example:     "bt!ping",
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        ping(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "feedback",
		Group:       group,
		Description: "Sends feedback to bot's author.",
		Usage:       "bt!feedback <your wall of text here>",
		Example:     "bt!feedback Damn your bot sucks!",
		Exec:        feedback(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "stats",
		Group:       group,
		Description: "Shows bot's runtime stats. First argument is 'general' by default.",
		Usage:       "bt!stats [general/artworks/commands]",
		Example:     "bt!stats",
		RateLimiter: gumi.NewRateLimiter(5 * time.Second),
		Exec:        stats(b),
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
		Exec:        artchannels(b),
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
		Exec:        addchannel(b),
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
		Exec:        removechannel(b),
	})
}

func help(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()

		eb.Title("Boe Tea's Documentation").Thumbnail(ctx.Session.State.User.AvatarURL(""))
		switch {
		case ctx.Args.Len() == 0:
			groups := make(map[string][]string)
			added := make(map[string]struct{})

			for _, cmd := range b.Router.Commands {
				if _, ok := added[cmd.Name]; ok {
					continue
				}

				_, ok := groups[cmd.Group]
				if !ok {
					groups[cmd.Group] = []string{cmd.Name}
					added[cmd.Name] = struct{}{}
					continue
				}

				groups[cmd.Group] = append(groups[cmd.Group], cmd.Name)
				added[cmd.Name] = struct{}{}
			}

			keys := make([]string, 0, len(groups))
			for key := range groups {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			for _, group := range groups {
				sort.Strings(group)
			}

			eb.Description(
				"This page shows bot's command groups. Under the group name you'll see a list of available commands. Use `bt!help <command name> for command's documentation.`",
			)

			for _, key := range keys {
				group := groups[key]

				eb.AddField(key, fmt.Sprintf(
					"```\n%v\n```", strings.Join(arrays.Map(group, func(s string) string {
						return "• " + s
					}), "\n"),
				), true)
			}
		case ctx.Args.Len() >= 1:
			name := ctx.Args.Get(0).Raw

			cmd, ok := b.Router.Commands[name]
			if !ok {
				return messages.HelpCommandNotFound(name)
			}

			var sb strings.Builder
			if cmd.GuildOnly {
				sb.WriteString("Guild only. ")
			}

			if cmd.NSFW {
				sb.WriteString("Only usable in NSFW channels. ")
			}

			eb.Description(sb.String())
			eb.AddField(
				"Description", "```"+cmd.Description+"```",
			)

			if len(cmd.Aliases) > 0 {
				eb.AddField(
					"Aliases", "```"+strings.Join(cmd.Aliases, " • ")+"```",
				)
			}

			eb.AddField(
				"Usage", "```"+cmd.Usage+"```",
			).AddField(
				"Example", "```"+cmd.Example+"```",
			)

			for name, desc := range cmd.Flags {
				eb.AddField(name, desc)
			}

			if cmd.RateLimiter != nil {
				eb.AddField("Cooldown", cmd.RateLimiter.Cooldown.String(), true)
			}

		}

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func ping(*bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()

		return ctx.ReplyEmbed(
			eb.Title("🏓 Pong!").AddField(
				"Heartbeat latency",
				ctx.Session.HeartbeatLatency().Round(time.Millisecond).String(),
			).Finalize(),
		)
	}
}

func about(*bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		locale := messages.AboutEmbed()

		eb := embeds.NewBuilder()
		eb.Title(locale.Title).Thumbnail(ctx.Session.State.User.AvatarURL(""))
		eb.Description(locale.Description)

		eb.AddField(
			locale.SupportServer,
			messages.ClickHere("https://discord.gg/hcxuHE7"),
			true,
		)

		eb.AddField(
			locale.InviteLink,
			messages.ClickHere(
				"https://discord.com/api/oauth2/authorize?client_id=636468907049353216&permissions=537259072&scope=bot",
			),
			true,
		)

		eb.AddField(
			locale.Patreon,
			messages.ClickHere("https://patreon.com/vtgare"),
			true,
		)

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func feedback(*bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() == 0 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		eb := embeds.NewBuilder()
		eb.Author(
			fmt.Sprintf("Feedback from %v", ctx.Event.Author.String()),
			"",
			ctx.Event.Author.AvatarURL(""),
		).Description(
			ctx.Args.Raw,
		).AddField(
			"Author Mention",
			ctx.Event.Author.Mention(),
			true,
		).AddField(
			"Author ID",
			ctx.Event.Author.ID,
			true,
		)

		if ctx.Event.GuildID != "" {
			eb.AddField(
				"Guild", ctx.Event.GuildID, true,
			)
		}

		if len(ctx.Event.Attachments) > 0 {
			att := ctx.Event.Attachments[0]
			if strings.HasSuffix(att.Filename, "png") ||
				strings.HasSuffix(att.Filename, "jpg") ||
				strings.HasSuffix(att.Filename, "gif") {
				eb.Image(att.URL)
			}
		}

		ch, err := ctx.Session.UserChannelCreate(ctx.Router.AuthorID)
		if err != nil {
			return err
		}

		_, err = ctx.Session.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
		if err != nil {
			return err
		}

		eb.Clear()

		reply := eb.SuccessTemplate("Feedback message has been sent.").Finalize()
		return ctx.ReplyEmbed(reply)
	}
}

func stats(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() == 0 {
			return generalStats(b, ctx)
		}

		arg := ctx.Args.Get(0).Raw
		switch arg {
		case "commands":
			return commandStats(b, ctx)
		case "artworks":
			return artworkStats(b, ctx)
		case "general":
			return generalStats(b, ctx)
		default:
			return messages.ErrIncorrectCmd(ctx.Command)
		}
	}
}

func generalStats(b *bot.Bot, ctx *gumi.Ctx) error {
	var (
		s   = ctx.Session
		mem runtime.MemStats
	)
	runtime.ReadMemStats(&mem)

	guilds := b.ShardManager.GuildCount()
	shards := b.ShardManager.ShardCount

	b.ShardManager.RLock()
	defer b.ShardManager.RUnlock()

	var channels int
	for _, shard := range b.ShardManager.Shards {
		for _, guild := range shard.Session.State.Guilds {
			channels += len(guild.Channels)
		}
	}

	latency := s.HeartbeatLatency().Round(1 * time.Millisecond)
	uptime := time.Since(b.StartTime).Round(1 * time.Second)

	_, totalArtworks := b.Stats.ArtworkStats()
	_, totalCommands := b.Stats.CommandStats()

	eb := embeds.NewBuilder()
	eb.Title("Bot stats")
	eb.AddField("Guilds", strconv.Itoa(guilds), true).
		AddField("Channels", strconv.Itoa(channels), true).
		AddField("Shards", strconv.Itoa(shards), true).
		AddField("Commands executed", strconv.FormatInt(totalCommands, 10), true).
		AddField("Artworks sent", strconv.FormatInt(totalArtworks, 10), true).
		AddField("Latency", latency.String(), true).
		AddField("Uptime", messages.FormatDuration(uptime), true).
		AddField("RAM used", fmt.Sprintf("%v MB", mem.Alloc/1024/1024), true)

	return ctx.ReplyEmbed(eb.Finalize())
}

func artworkStats(b *bot.Bot, ctx *gumi.Ctx) error {
	eb := embeds.NewBuilder()
	eb.Title("Artwork stats")

	stats, _ := b.Stats.ArtworkStats()
	for _, item := range stats {
		eb.AddField(item.Name, strconv.FormatInt(item.Count, 10))
	}

	return ctx.ReplyEmbed(eb.Finalize())
}

func commandStats(b *bot.Bot, ctx *gumi.Ctx) error {
	eb := embeds.NewBuilder()
	eb.Title("Command stats")

	stats, _ := b.Stats.CommandStats()
	for _, item := range stats {
		eb.AddField(item.Name, strconv.FormatInt(item.Count, 10), true)
	}

	return ctx.ReplyEmbed(eb.Finalize())
}

func set(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		showSettings := func() error {
			gd, err := ctx.Session.Guild(ctx.Event.GuildID)
			if err != nil {
				return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
			}

			guild, err := b.Store.Guild(context.Background(), gd.ID)
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

			guild, err := b.Store.Guild(context.Background(), ctx.Event.GuildID)
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
				if newSetting.Raw != string(store.GuildRepostEnabled) &&
					newSetting.Raw != string(store.GuildRepostDisabled) &&
					newSetting.Raw != string(store.GuildRepostStrict) {
					return messages.ErrUnknownRepostOption(newSetting.Raw)
				}

				oldSettingEmbed = guild.Repost
				newSettingEmbed = newSetting.Raw
				guild.Repost = store.GuildRepost(newSetting.Raw)
			case "repost.expiration":
				dur, err := time.ParseDuration(newSetting.Raw)
				if err != nil {
					return messages.ErrParseDuration(newSetting.Raw)
				}

				if dur < 1*time.Minute || dur > 168*time.Hour {
					return messages.ErrExpirationOutOfRange(newSetting.Raw)
				}

				oldSettingEmbed = guild.RepostExpiration
				newSettingEmbed = dur
				guild.RepostExpiration = dur
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

				oldSettingEmbed = guild.Crosspost
				newSettingEmbed = crosspost
				guild.Crosspost = crosspost
			case "reactions":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.Reactions
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
			case "deviant":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.Deviant
				newSettingEmbed = new
				guild.Deviant = new
			case "artstation":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.Artstation
				newSettingEmbed = new
				guild.Artstation = new
			case "tags":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.Tags
				newSettingEmbed = new
				guild.Tags = new
			case "footer":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.FlavorText
				newSettingEmbed = new
				guild.FlavorText = new
			case "twitter.skip":
				new, err := parseBool(newSetting.Raw)
				if err != nil {
					return err
				}

				oldSettingEmbed = guild.SkipFirst
				newSettingEmbed = new
				guild.SkipFirst = new
			default:
				return messages.ErrUnknownSetting(settingName.Raw)
			}

			_, err = b.Store.UpdateGuild(context.Background(), guild)
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

func artchannels(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		switch {
		case ctx.Args.Len() == 0:
			guild, err := b.Store.Guild(context.Background(), ctx.Event.GuildID)
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
					if _, err := b.Store.AddArtChannels(context.Background(), guildID, channels); err != nil {
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
					if _, err := b.Store.DeleteArtChannels(context.Background(), guildID, channels); err != nil {
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

			guild, err := b.Store.Guild(context.Background(), ctx.Event.GuildID)
			if err != nil {
				return messages.ErrGuildNotFound(err, ctx.Event.GuildID)
			}

			channels := make([]string, 0)
			for _, arg := range ctx.Args.Arguments[1:] {
				ch, err := ctx.Session.Channel(strings.Trim(arg.Raw, "<#>"))
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

func addchannel(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() == 0 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		guild, err := b.Store.Guild(context.Background(), ctx.Event.GuildID)
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
			context.Background(),
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

func removechannel(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() == 0 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		guild, err := b.Store.Guild(context.Background(), ctx.Event.GuildID)
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
			context.Background(),
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
	if s == "true" || s == "enabled" || s == "on" {
		return true, nil
	}

	if s == "false" || s == "disabled" || s == "off" {
		return false, nil
	}

	return false, messages.ErrParseBool(s)
}
