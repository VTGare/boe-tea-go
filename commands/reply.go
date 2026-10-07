package commands

import (
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/widget"
	"github.com/VTGare/gumi/v2"
	"github.com/disgoorg/disgo/discord"
)

// serveWidget sends the first page and serves button clicks until the
// widget stops, times out, or ctx ends.
func serveWidget(ctx *gumi.Context, b *bot.Bot, w *widget.Widget) error {
	pages := w.Pages
	if len(pages) == 0 {
		if ctx.IsInteraction() {
			return ctx.Reply(gumi.Text("No artworks found."))
		}

		return nil
	}

	if ctx.IsInteraction() {
		if err := ctx.Reply(&gumi.Response{Embeds: widget.Embeds(pages[:1]), Components: w.Controls()}); err != nil {
			return err
		}
	} else {
		if _, err := b.Sender.SendComplex(ctx.ChannelID(), discord.MessageCreate{
			Embeds:     widget.Embeds(pages[:1]),
			Components: w.Controls(),
		}); err != nil {
			return err
		}
	}

	return w.Serve(ctx.Context(), b.WidgetDispatcher)
}

// replyPages sends the pages as one message with page buttons.
func replyPages(ctx *gumi.Context, b *bot.Bot, pages []discord.Embed) error {
	pointers := make([]*discord.Embed, len(pages))
	for i := range pages {
		pointers[i] = &pages[i]
	}

	return serveWidget(ctx, b, widget.New(ctx.AuthorID(), pointers))
}

// runMessage returns the message that triggered the command, or a
// stand-in for slash commands, which have none.
func runMessage(ctx *gumi.Context) *discord.Message {
	if ctx.Message != nil {
		return ctx.Message
	}

	msg := &discord.Message{
		ID:        ctx.Interaction.ID(),
		ChannelID: ctx.ChannelID(),
		Member:    ctx.Member(),
	}

	if author := ctx.Author(); author != nil {
		msg.Author = *author
	}

	if guildID := ctx.GuildID(); guildID != 0 {
		msg.GuildID = &guildID
	}

	return msg
}
