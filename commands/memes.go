package commands

import (
	"bytes"
	"math/rand"
	"text/template"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/arrays"
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
>Something went inside {{.Ryo}}'s mouth
>{{.Amelia}} fed a chicken nugget to {{.Ryo}}
>{{.Ryo}} notices that the nuggie has this wet feeling, almost like a sauce but not exactly like one
>{{.Ryo}}: oi {{.Amelia}}! What did you feed me??
>{{.Amelia}} licks a noticeable drool on her lips
>{{.Amelia}}: It's a secret~`))

	whoisTemplate = template.Must(template.New("whois").Parse(
		"Who is {{.Faker}}? For the blind, He is the vision. " +
			"For the hungry, He is the chef. For the thirsty, He is the water. " +
			"If {{.Faker}} thinks, I agree. If {{.Faker}} speaks, I'm listening. " +
			"If {{.Faker}} has one fan, it is me. If {{.Faker}} has no fans, I don't exist.",
	))
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
		Name:        "whois",
		Group:       group,
		Description: "Who is Faker?",
		Usage:       "bt!whois <person 1>",
		Example:     "bt!whois Faker",
		Exec:        whois(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "gamba",
		Group:       group,
		Description: "GET IT TWISTED! 🗣️🗣️🗣️",
		Aliases:     []string{"gacha", "getittwisted"},
		Usage:       "bt!gamba",
		Example:     "bt!gamba",
		Exec:        gamba(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "cake",
		Group:       group,
		Description: "Local God eats cake",
		Aliases:     []string{},
		Usage:       "bt!cake",
		Example:     "bt!cake",
		Exec:        cake(b),
	})

	b.Router.RegisterCmd(&gumi.Command{
		Name:        "frenzy",
		Group:       group,
		Description: "The moon is red.",
		Aliases:     []string{},
		Usage:       "bt!frenzy",
		Example:     "bt!frenzy",
		Exec:        frenzy(b),
	})
}

func brainpower(*bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		return ctx.Reply(
			"O-oooooooooo AAAAE-A-A-I-A-U- JO-oooooooooooo AAE-O-A-A-U-U-A- " +
				"E-eee-ee-eee AAAAE-A-E-I-E-A-JO-ooo-oo-oo-oo EEEEO-A-AAA-AAAA",
		)
	}
}

func borgar(*bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()
		eb.Title("Cute dino girl enjoys borgar.").
			Description("🦕🍔").
			Image("https://i.kym-cdn.com/photos/images/original/001/568/282/ef2.gif")
		return ctx.ReplyEmbed(eb.Finalize())
	}
}

func nuggets(*bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if err := dgoutils.InitCommand(ctx, 2); err != nil {
			return err
		}

		n := &struct {
			Amelia string
			Ryo    string
		}{Amelia: ctx.Args.Get(1).Raw, Ryo: ctx.Args.Get(0).Raw}

		buf := new(bytes.Buffer)
		if err := nuggetsTemplatePart1.Execute(buf, n); err != nil {
			return err
		}

		return ctx.Reply(buf.String())
	}
}

func whois(*bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		if err := dgoutils.InitCommand(ctx, 1); err != nil {
			return err
		}

		n := &struct {
			Faker string
		}{Faker: ctx.Args.Get(0).Raw}

		buf := new(bytes.Buffer)
		if err := whoisTemplate.Execute(buf, n); err != nil {
			return err
		}

		return ctx.Reply(buf.String())
	}
}

func gamba(*bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		getItTwisted := rand.Intn(10) != 0

		var text string
		if getItTwisted {
			text = `🦍 🗣 GET IT TWISTED 🌪 , GAMBLE ✅ . PLEASE START GAMBLING 👍 . GAMBLING IS AN INVESTMENT 🎰 AND AN INVESTMENT ONLY 👍 . YOU WILL PROFIT 💰 , YOU WILL WIN ❗ ️. YOU WILL DO ALL OF THAT 💯 , YOU UNDERSTAND ⁉ ️ YOU WILL BECOME A BILLIONAIRE 💵 📈 AND REBUILD YOUR FUCKING LIFE 🤯`
		} else {
			text = `🦍 🗣️ DO NOT GET IT TWISTED 🌪️ , DO NOT GAMBLE 🚫 . DO NOT START GAMBLING ❌ . GAMBLING IS ENTERTAINMENT 🎰 AND ENTERTAINMENT ONLY 👍 . YOU WONT BREAK EVEN 🛑 , YOU WONT WIN ⚠️ ️. YOU WONT DO ANY OF THAT 💯 , YOU UNDERSTAND ⁉️ ️ YOU WILL ONLY GO INTO DEBT 💵 📉 AND RUIN YOUR FUCKING LIFE 😵`
		}

		return ctx.Reply(text)
	}
}

func cake(*bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		eb := embeds.NewBuilder()
		eb.Title("Local God eats cake").
			Description("🙏🍰").
			Image("https://cdn.discordapp.com/attachments/1129829799179853914/1160609012115578970/haruhi-haruhi-suzumiya.gif")

		return ctx.ReplyEmbed(eb.Finalize())
	}
}

var frenzyImages = []string{
	"https://media.tenor.com/wLs85BqnUJUAAAAC/eminence-in-shadow-shadow.gif",
	"https://media.tenor.com/otI2t29q9RYAAAAC/the-eminence-in-shadow-the-eminence-of-shadow.gif",
	"https://media.tenor.com/YmcBY9nlAwsAAAAC/cid-kagenou-eminence-in-shadow.gif",
	"https://static.wikia.nocookie.net/to-be-a-power-in-the-shadows/images/9/9f/Red_Moon-_Anime.png",
}

func frenzy(*bot.Bot) func(ctx *gumi.Ctx) error {
	return func(ctx *gumi.Ctx) error {
		image := arrays.RandomElement(frenzyImages)

		eb := embeds.NewBuilder()
		eb.Title("🔴 The moon is red 🔴").
			Description("*The frenzy has begun.* 🤪\n*We're out of time.* ⏳\n*Run if you value your life.* 🏃").
			Color(0x880808).
			Image(*image)

		return ctx.ReplyEmbed(eb.Finalize())
	}
}
