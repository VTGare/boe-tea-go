package commands

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/boe-tea-go/router"
	"github.com/VTGare/embeds"
)

func generalGroup(b *bot.Bot) []*router.Command {
	return []*router.Command{
		{
			Name:        "about",
			Category:    "General",
			Aliases:     []string{"invite", "patreon", "support"},
			Description: "Bot's about page with the invite link and other useful stuff.",
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 5*time.Second),
			Examples:    []string{"about"},
			Handler:     about(b),
		},
		{
			Name:        "ping",
			Category:    "General",
			Description: "Checks bot's availability and response time.",
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 5*time.Second),
			Examples:    []string{"ping"},
			Handler:     ping(b),
		},
		{
			Name:        "feedback",
			Category:    "General",
			Description: "Sends feedback to bot's author.",
			Options: []*router.Option{
				router.String("text", "Your feedback").Require().Greedy(),
				router.Attachment("image", "Screenshot to attach"),
			},
			Examples: []string{"feedback Damn your bot sucks!"},
			Handler:  feedback(b),
		},
		{
			Name:        "stats",
			Category:    "General",
			Description: "Shows bot's runtime stats.",
			Cooldown:    router.NewCooldown(router.CooldownUser, 1, 5*time.Second),
			Options: []*router.Option{
				router.String("section", "Which stats to show").WithChoices(
					router.Choice{Name: "general", Value: "general"},
					router.Choice{Name: "artworks", Value: "artworks"},
					router.Choice{Name: "commands", Value: "commands"},
				),
			},
			Examples: []string{"stats", "stats artworks"},
			Handler:  statsCommand(b),
		},
	}
}

func about(*bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
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

		return ctx.Reply(router.Embed(eb.Finalize()))
	}
}

func ping(*bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		eb := embeds.NewBuilder()

		return ctx.Reply(router.Embed(
			eb.Title("🏓 Pong!").AddField(
				"Heartbeat latency",
				ctx.Session.HeartbeatLatency().Round(time.Millisecond).String(),
			).Finalize(),
		))
	}
}

func feedback(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		author := ctx.Author()

		eb := embeds.NewBuilder()
		eb.Author(
			fmt.Sprintf("Feedback from %v", author.String()),
			"",
			author.AvatarURL(""),
		).Description(
			ctx.Options.String("text"),
		).AddField(
			"Author Mention",
			author.Mention(),
			true,
		).AddField(
			"Author ID",
			author.ID,
			true,
		)

		if ctx.GuildID() != "" {
			eb.AddField(
				"Guild", ctx.GuildID(), true,
			)
		}

		var imageURL, imageName string
		if ctx.IsMessage() && len(ctx.Message.Attachments) > 0 {
			imageURL = ctx.Message.Attachments[0].URL
			imageName = ctx.Message.Attachments[0].Filename
		} else if att := ctx.Options.Attachment("image"); att != nil {
			imageURL = att.URL
			imageName = att.Filename
		}

		for _, suffix := range []string{"png", "jpg", "jpeg", "gif"} {
			if imageURL != "" && strings.HasSuffix(imageName, suffix) {
				eb.Image(imageURL)
			}
		}

		ch, err := ctx.Session.UserChannelCreate(b.Config.Discord.AuthorID)
		if err != nil {
			return err
		}

		_, err = ctx.Session.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
		if err != nil {
			return err
		}

		eb.Clear()

		return ctx.Reply(router.Embed(eb.SuccessTemplate("Feedback message has been sent.").Finalize()))
	}
}

func statsCommand(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		switch ctx.Options.String("section") {
		case "", "general":
			return generalStats(b, ctx)
		case "commands":
			return commandStats(b, ctx)
		case "artworks":
			return artworkStats(b, ctx)
		default:
			return messages.ErrIncorrectCmd(ctx.Command)
		}
	}
}

func generalStats(b *bot.Bot, ctx *router.Context) error {
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

	return ctx.Reply(router.Embed(eb.Finalize()))
}

func artworkStats(b *bot.Bot, ctx *router.Context) error {
	eb := embeds.NewBuilder()
	eb.Title("Artwork stats")

	stats, _ := b.Stats.ArtworkStats()
	for _, item := range stats {
		eb.AddField(item.Name, strconv.FormatInt(item.Count, 10))
	}

	return ctx.Reply(router.Embed(eb.Finalize()))
}

func commandStats(b *bot.Bot, ctx *router.Context) error {
	eb := embeds.NewBuilder()
	eb.Title("Command stats")

	stats, _ := b.Stats.CommandStats()
	for _, item := range stats {
		eb.AddField(item.Name, strconv.FormatInt(item.Count, 10), true)
	}

	return ctx.Reply(router.Embed(eb.Finalize()))
}
