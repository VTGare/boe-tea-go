package commands

import (
	"bytes"
	"text/template"
	"time"

	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/VTGare/boe-tea-go/pkg/messages"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
)

var (
	nuggetsTemplate = template.Must(template.New("nuggets").Parse(`>{{.Ryo}} and {{.Amelia}} sits side by side watching a movie
>They're eating pack of chicken nuggets while watching
>Somehow the takeout didn't include much bbq sauce except for a half filled one
>Sauce has been depleted after {{.Ryo}} has eaten 3 nuggets
>{{.Ryo}}: {{.Amelia}}, we're out of sauce. Do you have any in your fridge?
>{{.Amelia}}: No sorry, I don't use any sauce
>{{.Ryo}} tries to eat the nuggets without any sauce but she has a hard time enjoying them
>{{.Amelia}} notices the predicament of {{.Ryo}}
>{{.Amelia}}: {{.Ryo}}, say "aahhh"~~
>{{.Ryo}}: a--- mmm!!??????
>Something went inside {{.Ryo}}’s mouth
>{{.Amelia}} fed a chicken nugget to {{.Ryo}}
>{{.Ryo}} notices that the nuggie has this wet feeling, almost like a sauce but not exactly like one
>{{.Ryo}}: oi {{.Amelia}}! What did you feed me??
>{{.Amelia}} licks a noticeable drool on her lips
>{{.Amelia}}: It's a secret~`))
)

func memesGroup(b *bot.Bot) {
	group := "memes"

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "brainpower",
		Group:       group,
		Description: "Adrenaline is pumping.",
		Usage:       "You know how to use it.",
		Example:     "catJAM",
		RateLimiter: gumi.NewRateLimiter(15 * time.Second),
		Exec:        brainpower(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "borgar",
		Group:       group,
		Description: "Cute dino girl eats borgar.",
		Usage:       "You know how to use it.",
		Example:     "Actually I don't know how to use it.",
		Exec:        borgar(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "nuggets",
		Group:       group,
		Description: "Create ships by feeding nuggets.",
		Usage:       "bt!nuggets <person 1> <person 2>",
		Example:     "bt!nuggets 2B 9S",
		Exec:        nuggets(b),
	})
}

func brainpower(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		return ctx.Reply(
			"O-oooooooooo AAAAE-A-A-I-A-U- JO-oooooooooooo AAE-O-A-A-U-U-A- E-eee-ee-eee AAAAE-A-E-I-E-A-JO-ooo-oo-oo-oo EEEEO-A-AAA-AAAA",
		)
	}
}

func borgar(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()
		eb.Title(
			"Cute dino girl enjoys borgar.",
		).Description(
			"🦕🍔",
		).Image(
			"https://images-ext-2.discordapp.net/external/gRgdT4gZIPbY26qK9iM0edWQA4hYPZF5RvxVdSeXhRQ/https/i.kym-cdn.com/photos/images/original/001/568/282/ef2.gif",
		)
		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func nuggets(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() < 2 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		n := &struct {
			Amelia string
			Ryo    string
		}{Amelia: ctx.Args.Get(1).Raw, Ryo: ctx.Args.Get(0).Raw}

		buf := new(bytes.Buffer)
		nuggetsTemplate.Execute(buf, n)

		ctx.Reply(buf.String())
		return nil
	}
}
