package commands

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/gumi/v2"
)

func generalGroup(b *bot.Bot) []*gumi.Command {
	return []*gumi.Command{
		{
			Name:        "about",
			Category:    "General",
			Aliases:     []string{"invite", "patreon", "support"},
			Description: "Bot's about page with the invite link and other useful stuff.",
			Cooldown:    gumi.NewCooldown(gumi.CooldownUser, 1, 5*time.Second),
			Examples:    []string{"about"},
			Handler:     about(b),
		},
		{
			Name:        "ping",
			Category:    "General",
			Description: "Checks bot's availability and response time.",
			Cooldown:    gumi.NewCooldown(gumi.CooldownUser, 1, 5*time.Second),
			Examples:    []string{"ping"},
			Handler:     ping(b),
		},
		{
			Name:        "feedback",
			Category:    "General",
			Description: "Sends feedback to bot's author.",
			Options: []*gumi.Option{
				gumi.String("text", "Your feedback").Require().Greedy(),
				gumi.Attachment("image", "Screenshot to attach"),
			},
			Examples: []string{"feedback Damn your bot sucks!"},
			Handler:  feedback(b),
		},
		{
			Name:        "stats",
			Category:    "General",
			Description: "Shows bot's runtime stats.",
			Cooldown:    gumi.NewCooldown(gumi.CooldownUser, 1, 5*time.Second),
			Options: []*gumi.Option{
				gumi.String("section", "Which stats to show").WithChoices(
					gumi.Choice{Name: "general", Value: "general"},
					gumi.Choice{Name: "artworks", Value: "artworks"},
					gumi.Choice{Name: "commands", Value: "commands"},
				),
			},
			Examples: []string{"stats", "stats artworks"},
			Handler:  statsCommand(b),
		},
	}
}

func about(b *bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		locale := messages.AboutEmbed()

		eb := embeds.NewBuilder()
		eb.Title(locale.Title).Thumbnail(b.AvatarURL())
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

		return ctx.Reply(gumi.Embed(eb.Finalize()))
	}
}

func ping(b *bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		eb := embeds.NewBuilder()

		return ctx.Reply(gumi.Embed(
			eb.Title("🏓 Pong!").AddField(
				"Heartbeat latency",
				b.Latency(ctx.GuildID()).Round(time.Millisecond).String(),
			).Finalize(),
		))
	}
}

func feedback(b *bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		author := ctx.Author()

		eb := embeds.NewBuilder()
		eb.Author(
			fmt.Sprintf("Feedback from %v", author.Username),
			"",
			author.EffectiveAvatarURL(),
		).Description(
			ctx.Options.String("text"),
		).AddField(
			"Author Mention",
			author.Mention(),
			true,
		).AddField(
			"Author ID",
			author.ID.String(),
			true,
		)

		if ctx.GuildID() != 0 {
			eb.AddField(
				"Guild", ctx.GuildID().String(), true,
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

		if err := dgoutils.SendDM(ctx.Client, dgoutils.ParseID(b.Config.Discord.AuthorID), eb.Finalize()); err != nil {
			return err
		}

		eb.Clear()

		return ctx.Reply(gumi.Embed(eb.SuccessTemplate("Feedback message has been sent.").Finalize()))
	}
}

func statsCommand(b *bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
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

func generalStats(b *bot.Bot, ctx *gumi.Context) error {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)

	guilds := ctx.Client.Caches.GuildsLen()
	channels := ctx.Client.Caches.ChannelsLen()
	shards := b.ShardCount()

	latency := b.Latency(ctx.GuildID()).Round(1 * time.Millisecond)
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

	return ctx.Reply(gumi.Embed(eb.Finalize()))
}

func artworkStats(b *bot.Bot, ctx *gumi.Context) error {
	eb := embeds.NewBuilder()
	eb.Title("Artwork stats")

	stats, _ := b.Stats.ArtworkStats()
	for _, item := range stats {
		eb.AddField(item.Name, strconv.FormatInt(item.Count, 10))
	}

	return ctx.Reply(gumi.Embed(eb.Finalize()))
}

func commandStats(b *bot.Bot, ctx *gumi.Context) error {
	eb := embeds.NewBuilder()
	eb.Title("Command stats")

	stats, _ := b.Stats.CommandStats()
	for _, item := range stats {
		eb.AddField(item.Name, strconv.FormatInt(item.Count, 10), true)
	}

	return ctx.Reply(gumi.Embed(eb.Finalize()))
}
