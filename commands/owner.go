package commands

import (
	"strings"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/messages"
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
		Example:     "bt!reply You know who else is shit? Yrou'e mom in bed :^)",
		AuthorOnly:  true,
		Exec:        reply(b),
	})
}

func reply(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if err := dgoutils.InitCommand(ctx, 2); err != nil {
			return err
		}

		userID := dgoutils.Trimmer(ctx, 0)

		s := b.ShardManager.SessionForDM()
		ch, err := s.UserChannelCreate(userID)
		if err != nil {
			return err
		}

		eb := embeds.NewBuilder()
		reply := strings.TrimPrefix(strings.TrimSpace(ctx.Args.Raw), ctx.Args.Get(0).Raw)

		eb.Author("Feedback reply", "", ctx.Session.State.User.AvatarURL("")).
			Description(reply)

		if attachments := ctx.Event.Attachments; len(attachments) >= 1 {
			if strings.HasSuffix(attachments[0].Filename, "png") ||
				strings.HasSuffix(attachments[0].Filename, "jpg") ||
				strings.HasSuffix(attachments[0].Filename, "gif") {
				eb.Image(attachments[0].URL)
			}
		}

		_, err = s.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
		if err != nil {
			return err
		}

		eb.Clear()
		return ctx.ReplyEmbed(eb.SuccessTemplate("Reply has been sent.").Finalize())
	}
}
