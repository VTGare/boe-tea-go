package commands

import (
	"strings"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/router"
	"github.com/VTGare/embeds"
)

func ownerGroup(b *bot.Bot) []*router.Command {
	return []*router.Command{
		{
			Name:        "reply",
			Category:    "Owner",
			Description: "Owner's command to reply to feedback",
			Checks:      []router.Check{router.OwnerOnly},
			Hidden:      true,
			Options: []*router.Option{
				router.String("user", "User to reply to").Require(),
				router.String("message", "Reply text").Require().Greedy(),
				router.Attachment("image", "Image to attach"),
			},
			Examples: []string{"reply 1234567890 Thanks for the feedback!"},
			Handler:  reply(b),
		},
	}
}

func reply(b *bot.Bot) router.Handler {
	return func(ctx *router.Context) error {
		userID := dgoutils.TrimmerRaw(ctx.Options.String("user"))

		s := b.ShardManager.SessionForDM()
		ch, err := s.UserChannelCreate(userID)
		if err != nil {
			return err
		}

		eb := embeds.NewBuilder()
		reply := ctx.Options.String("message")

		eb.Author("Feedback reply", "", ctx.Session.State.User.AvatarURL("")).
			Description(reply)

		var imageURL, imageName string
		if ctx.IsMessage() && len(ctx.Message.Attachments) > 0 {
			imageURL = ctx.Message.Attachments[0].URL
			imageName = ctx.Message.Attachments[0].Filename
		} else if att := ctx.Options.Attachment("image"); att != nil {
			imageURL = att.URL
			imageName = att.Filename
		}

		if imageURL != "" && (strings.HasSuffix(imageName, "png") ||
			strings.HasSuffix(imageName, "jpg") ||
			strings.HasSuffix(imageName, "gif")) {
			eb.Image(imageURL)
		}

		_, err = s.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
		if err != nil {
			return err
		}

		eb.Clear()
		return ctx.Reply(router.Embed(eb.SuccessTemplate("Reply has been sent.").Finalize()))
	}
}
