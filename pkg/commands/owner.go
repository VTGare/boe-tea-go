package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/VTGare/boe-tea-go/internal/arrays"
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

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "test",
		Group:       group,
		Description: "For testing purposes",
		Usage:       "Don't",
		Example:     "You can't",
		AuthorOnly:  true,
		Exec:        testChannel(b),
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

func testChannel(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() == 0 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		channelID := strings.Trim(ctx.Args.Get(0).Raw, "<#>")
		ch, err := ctx.Session.Channel(channelID)
		if err != nil {
			return err
		}

		guild, err := b.Guilds.FindOne(context.Background(), ch.GuildID)
		if err != nil {
			return err
		}

		tests := []func() (string, bool){
			func() (string, bool) {
				if len(guild.ArtChannels) == 0 || arrays.AnyString(guild.ArtChannels, channelID) {
					return "The channel is located in art channels or art channels are empty.", true
				}

				return "Artworks won't be sent", false
			},
			func() (string, bool) {
				url := "https://pixiv.net/artworks/90260843"
				for _, provider := range b.ArtworkProviders {
					if id, ok := provider.Match(url); ok {
						artwork, err := provider.Find(id)
						if err != nil {
							return err.Error(), false
						}

						sends, err := artwork.MessageSends("This is just a test :^)")
						if err != nil {
							return err.Error(), false
						}

						if len(sends) > 0 {
							err = ctx.ReplyTextEmbed(sends[0].Content, sends[0].Embed)
							if err != nil {
								return err.Error(), false
							}
						}
					}
				}

				return "Artwork was sent successfully.", true
			},
		}

		sb := strings.Builder{}
		for ind, test := range tests {
			msg, res := test()

			emoji := "❌"
			if res {
				emoji = "✔"
			}

			sb.WriteString(
				fmt.Sprintf("#%v %v | Message: %v\n", ind+1, emoji, msg),
			)
		}

		eb := embeds.NewBuilder()
		eb.Title("Test results")
		eb.Description(sb.String())

		return ctx.ReplyEmbed(eb.Finalize())
	}
}
