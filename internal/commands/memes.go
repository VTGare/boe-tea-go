package commands

import (
	"bytes"
	"text/template"

	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/utils"
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

func init() {
	groupName := "memes"

	Commands = append(Commands, &gumi.Command{
		Name:        "borgar",
		Description: "Dino eats borgar.",
		Group:       groupName,
		Usage:       "",
		Example:     "",
		Exec:        borgar,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "minesweeper",
		Description: "Budget minesweeper.",
		Group:       groupName,
		Usage:       "",
		Example:     "",
		Exec:        minesweeper,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "nuggets",
		Description: "Nuggets copypasta.",
		Group:       groupName,
		Usage:       "",
		Example:     "",
		Exec:        nuggets,
	})
	Commands = append(Commands, &gumi.Command{
		Name:        "brainpower",
		Description: "E E E x30 ADRENALINE IS PUMPING! x2",
		Group:       groupName,
		Usage:       "",
		Example:     "",
		Exec:        brainpower,
	})
}

func borgar(ctx *gumi.Ctx) error {
	eb := embeds.NewBuilder().Title("🦕🍔").Image("https://images-ext-2.discordapp.net/external/gRgdT4gZIPbY26qK9iM0edWQA4hYPZF5RvxVdSeXhRQ/https/i.kym-cdn.com/photos/images/original/001/568/282/ef2.gif?width=438&height=444")

	ctx.ReplyEmbed(eb.Finalize())
	return nil
}

//NuggetsCopypasta ...
type NuggetsCopypasta struct {
	Amelia string
	Ryo    string
}

func nuggets(ctx *gumi.Ctx) error {
	if ctx.Args.Len() < 2 {
		return utils.ErrNotEnoughArguments
	}

	n := &NuggetsCopypasta{Amelia: ctx.Args.Get(1).Raw, Ryo: ctx.Args.Get(0).Raw}

	buf := new(bytes.Buffer)
	nuggetsTemplate.Execute(buf, n)

	ctx.Reply(buf.String())
	return nil
}

func brainpower(ctx *gumi.Ctx) error {
	ctx.Reply("O-oooooooooo AAAAE-A-A-I-A-U- JO-oooooooooooo AAE-O-A-A-U-U-A- E-eee-ee-eee AAAAE-A-E-I-E-A-JO-ooo-oo-oo-oo EEEEO-A-AAA-AAAA")
	return nil
}

func minesweeper(ctx *gumi.Ctx) error {
	ms := &Minesweeper{}
	ms.generateField()

	ctx.Reply(ms.String())
	return nil
}
