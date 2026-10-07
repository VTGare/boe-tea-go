package commands

import (
	"strings"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/gumi/v2"
)

func ownerGroup(b *bot.Bot) []*gumi.Command {
	return []*gumi.Command{
		{
			Name:        "reply",
			Category:    "Owner",
			Description: "Owner's command to reply to feedback",
			Checks:      []gumi.Check{gumi.OwnerOnly},
			Hidden:      true,
			Options: []*gumi.Option{
				gumi.String("user", "User to reply to").Require(),
				gumi.String("message", "Reply text").Require().Greedy(),
				gumi.Attachment("image", "Image to attach"),
			},
			Examples: []string{"reply 1234567890 Thanks for the feedback!"},
			Handler:  reply(b),
		},
	}
}

func reply(b *bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		userID := dgoutils.ParseID(dgoutils.TrimmerRaw(ctx.Options.String("user")))
		if userID == 0 {
			return gumi.Errorf("`%s` isn't a user ID.", ctx.Options.String("user"))
		}

		eb := embeds.NewBuilder()
		reply := ctx.Options.String("message")

		eb.Author("Feedback reply", "", b.AvatarURL()).
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

		if err := dgoutils.SendDM(ctx.Client, userID, eb.Finalize()); err != nil {
			return err
		}

		eb.Clear()
		return ctx.Reply(gumi.Embed(eb.SuccessTemplate("Reply has been sent.").Finalize()))
	}
}
