package commands

import (
	"fmt"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"
)

func generalGroup(b *bot.Bot) {
	group := "general"

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

func feedback(*bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if err := dgoutils.InitCommand(ctx, 1); err != nil {
			return err
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
