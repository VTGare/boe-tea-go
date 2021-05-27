package commands

import (
	"strings"

	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/messages"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
)

func ownerGroup(b *bot.Bot) {
	group := "owner"

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "reply",
		Group:       group,
		Description: "Owner's command to reply to feedback",
		Usage:       "bt!reply <wall of text>",
		Example:     "bt!reply You know who else is shit? Your momma :)",
		AuthorOnly:  true,
		Exec:        reply(b),
	})
}

func reply(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() < 2 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		userID := strings.Trim(ctx.Args.Get(0).Raw, "<@!>")
		ch, err := ctx.Session.UserChannelCreate(userID)
		if err != nil {
			return err
		}

		eb := embeds.NewBuilder()

		eb.Author(
			"Feedback reply",
			"",
			ctx.Session.State.User.AvatarURL(""),
		).Description(
			strings.TrimPrefix(
				strings.TrimSpace(ctx.Args.Raw),
				userID,
			),
		)

		if attachments := ctx.Event.Attachments; len(attachments) >= 1 {
			if strings.HasSuffix(attachments[0].Filename, "png") || strings.HasSuffix(attachments[0].Filename, "jpg") || strings.HasSuffix(attachments[0].Filename, "gif") {
				eb.Image(attachments[0].URL)
			}
		}

		_, err = ctx.Session.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
		if err != nil {
			return err
		}

		eb.Clear()
		ctx.ReplyEmbed(eb.SuccessTemplate("Reply has been sent.").Finalize())
		return nil
	}
}
