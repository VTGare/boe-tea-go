package commands

import (
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/widget"
	"github.com/VTGare/boe-tea-go/router"
	"github.com/bwmarrin/discordgo"
)

// serveWidget sends the first page and serves button clicks until the
// widget stops, times out, or ctx ends.
func serveWidget(ctx *router.Context, b *bot.Bot, w *widget.Widget) error {
	pages := w.Pages
	if len(pages) == 0 {
		if ctx.IsInteraction() {
			return ctx.Reply(router.Text("No artworks found."))
		}

		return nil
	}

	if ctx.IsInteraction() {
		if err := ctx.Reply(&router.Response{Embeds: pages[:1], Components: w.Controls()}); err != nil {
			return err
		}
	} else {
		if _, err := b.Sender.SendComplex(ctx.GuildID(), ctx.ChannelID(), &discordgo.MessageSend{
			Embeds:     pages[:1],
			Components: w.Controls(),
		}); err != nil {
			return err
		}
	}

	return w.Serve(ctx.Context(), b.WidgetDispatcher)
}

// replyPages sends the pages as one message with page buttons.
func replyPages(ctx *router.Context, b *bot.Bot, pages []*discordgo.MessageEmbed) error {
	return serveWidget(ctx, b, widget.New(ctx.AuthorID(), pages))
}

// runMessage returns the message that triggered the command, or a
// stand-in for slash commands, which have none.
func runMessage(ctx *router.Context) *discordgo.Message {
	if ctx.Message != nil {
		return ctx.Message
	}

	return &discordgo.Message{
		ID:        ctx.Interaction.ID,
		ChannelID: ctx.ChannelID(),
		GuildID:   ctx.GuildID(),
		Author:    ctx.Author(),
		Member:    ctx.Member(),
	}
}
