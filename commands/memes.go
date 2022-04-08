package commands

import (
	"bytes"
	"text/template"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/messages"
	"github.com/VTGare/embeds"
	"github.com/VTGare/gumi"
)

var (
	nuggetsTemplatePart1 = template.Must(template.New("nuggets").Parse(
		`>{{.Ryo}} and {{.Amelia}} sits side by side watching a movie
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

	nuggetsTemplatePart2 = template.Must(template.New("nuggets2").Parse(
		`The smell of roasted beans fills the air inside a building with plastered white walls engraved with vine-like looking aesthetics. Potted plants lined up outside a humongous window of this one-story building. Outside the window shows a street parked with cars, varying vehicles being driven, and pedestrians walking on their lanes, crossing modern city streets.
This window was engraved with the letters, "ADB Cafe"
The placed was almost crowded. It was a weekend after all. It was one of Holocity's trending establishments.
"Teamates only!" The manager dressed in a yellow shirt, black pants and black leather shoes told a customer in line. It didn't take long for a security guard to approach this unwelcomed guest.
Among this crowd, sits {{.Shinaro}}. He was muttering... contemplating—reflecting on what words to say next. He kept talking to himself for a while now. Twenty minutes have passed from the time he agreed on with the person he was meeting.
"We will..." 
He stuttered. 
"No," He changed his mind. "Let's have a nice cup of coffee together," He finally settled with what he'll say next. He thought, that it had to be perfect.
This meeting... it has to be perfect. He thought.
After all, he was meeting someone special.
A bell chimed as a person entered the door. {{.Shinaro}} turned his head to look.
A woman with blonde hair, in a detective's cap and a monocle-style hairpin entered. She was happy seeing the teamates enjoying the cafe. As she walked in, the customer being warned by the manager was already being forcefully kicked out the door that was already behind her.
She paid no heed to this as she entered hiccuping.
{{.Shinaro}} turned his head away from this woman. She wasn't the one he was looking for. He waited, looking at his golden watch as the time continues to pass just to see this person he wanted to see the most.`))

	nuggetsTemplatePart3 = template.Must(template.New("nuggets3").Parse(
		`The cafe's bell chimed once more. {{.Shinaro}} turned to look, eager to finally meet the person he wanted to see for so long.
A man with white hair, dressed in an all-black shirt, pants, tie and shoes in a detective's cap entered. He had a plastic bag. Who knows what is inside it.
It was {{.VT}}.
{{.Shinaro}}'s pupils dilated upon seeing this man.
{{.VT}} looked around the crowded cafe. It didn't take long for him to find {{.Shinaro}}. He smiled at him.
{{.VT}} walked to {{.Shinaro}}'s table. He sat down with him on a chair across the table they reserved.
"You're late." {{.Shinaro}} said.
"I know."
{{.Shinaro}} was worried. He practiced and thought of what to say for so long, but the words he planned to say at first didn't come to his mind.
"So, to apologize for making you wait this long, I brought a little something," {{.VT}} placed the plastic bag on the table, and pulled out a red and white box that was labeled "KFP"
He opened the box, to present its contents.
"N-Nuggets...?" {{.Shinaro}} said.
"Yes." {{.VT}} said with the chicken nugget held on his fingertips, placed near his mouth, as he stared intently into {{.Shinaro}}'s eyes.`))
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

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "nuggets2",
		Group:       group,
		Description: "Long awaited nuggets sequel.",
		Usage:       "bt!nuggets2 <person 1> <person 2>",
		Example:     "bt!nuggets2 vt Shinaro",
		Exec:        nuggets2(b),
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
		nuggetsTemplatePart1.Execute(buf, n)

		ctx.Reply(buf.String())
		return nil
	}
}

func nuggets2(b *bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if ctx.Args.Len() < 2 {
			return messages.ErrIncorrectCmd(ctx.Command)
		}

		n := &struct {
			VT      string
			Shinaro string
		}{VT: ctx.Args.Get(0).Raw, Shinaro: ctx.Args.Get(1).Raw}

		buf := new(bytes.Buffer)
		nuggetsTemplatePart2.Execute(buf, n)

		part2 := buf.String()
		buf.Reset()

		nuggetsTemplatePart3.Execute(buf, n)
		part3 := buf.String()

		reply := func() error {
			if err := ctx.Reply(part2); err != nil {
				return err
			}

			if err := ctx.Reply(part3); err != nil {
				return err
			}

			return nil
		}

		return reply()
	}
}
