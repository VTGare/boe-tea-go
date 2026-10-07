package commands

import (
	"bytes"
	"math/rand"
	"text/template"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/arrays"
	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/gumi/v2"
	"github.com/julien040/go-ternary"
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

func memesGroup(b *bot.Bot) []*gumi.Command {
	return []*gumi.Command{
		{
			Name:        "brainpower",
			Category:    "Memes",
			Description: "Adrenaline is pumping.",
			Cooldown:    gumi.NewCooldown(gumi.CooldownUser, 1, 15*time.Second),
			Examples:    []string{"brainpower"},
			Handler:     brainPower(b),
		},
		{
			Name:        "borgar",
			Category:    "Memes",
			Description: "Cute dino girl eats borgar.",
			Examples:    []string{"borgar"},
			Handler:     borgar(b),
		},
		{
			Name:        "nuggets",
			Category:    "Memes",
			Description: "Create ships by feeding nuggets.",
			Options: []*gumi.Option{
				gumi.String("person1", "First person").Require(),
				gumi.String("person2", "Second person").Require(),
			},
			Examples: []string{"nuggets 2B 9S"},
			Handler:  nuggets(b),
		},
		{
			Name:        "whois",
			Category:    "Memes",
			Description: "Who is Faker?",
			Options: []*gumi.Option{
				gumi.String("person", "Person to identify").Require(),
			},
			Examples: []string{"whois Faker"},
			Handler:  whoIs(b),
		},
		{
			Name:        "gamba",
			Category:    "Memes",
			Description: "GET IT TWISTED! 🗣️🗣️🗣️",
			Aliases:     []string{"gacha", "getittwisted"},
			Examples:    []string{"gamba"},
			Handler:     gamba(b),
		},
		{
			Name:        "cake",
			Category:    "Memes",
			Description: "Local God eats cake",
			Examples:    []string{"cake"},
			Handler:     cake(b),
		},
		{
			Name:        "frenzy",
			Category:    "Memes",
			Description: "The moon is red.",
			Examples:    []string{"frenzy"},
			Handler:     frenzy(b),
		},
		{
			Name:        "kaiki",
			Category:    "Memes",
			Description: "Introduce yourself as a guy who always lies",
			Aliases:     []string{"lie", "trick"},
			Examples:    []string{"kaiki"},
			Handler:     kaiki(b),
		},
	}
}

func brainPower(*bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		return ctx.Reply(gumi.Text(
			"O-oooooooooo AAAAE-A-A-I-A-U- JO-oooooooooooo AAE-O-A-A-U-U-A- " +
				"E-eee-ee-eee AAAAE-A-E-I-E-A-JO-ooo-oo-oo-oo EEEEO-A-AAA-AAAA",
		))
	}
}

func borgar(*bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		eb := embeds.NewBuilder()
		eb.Title("Cute dino girl enjoys borgar.").
			Description("🦕🍔").
			Image("https://i.kym-cdn.com/photos/images/original/001/568/282/ef2.gif")
		return ctx.Reply(gumi.Embed(eb.Finalize()))
	}
}

func nuggets(*bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		n := &struct {
			Amelia string
			Ryo    string
		}{Amelia: ctx.Options.String("person2"), Ryo: ctx.Options.String("person1")}

		buf := new(bytes.Buffer)
		if err := nuggetsTemplatePart1.Execute(buf, n); err != nil {
			return err
		}

		return ctx.Reply(gumi.Text(buf.String()))
	}
}

func whoIs(*bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		n := &struct {
			Faker string
		}{Faker: ctx.Options.String("person")}

		buf := new(bytes.Buffer)
		if err := whoisTemplate.Execute(buf, n); err != nil {
			return err
		}

		return ctx.Reply(gumi.Text(buf.String()))
	}
}

func gamba(*bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		getItTwisted := rand.Intn(10) != 0

		text := ternary.If(getItTwisted,
			`🦍 🗣 GET IT TWISTED 🌪 , GAMBLE ✅ . PLEASE START GAMBLING 👍 . GAMBLING IS AN INVESTMENT 🎰 AND AN INVESTMENT ONLY 👍 . YOU WILL PROFIT 💰 , YOU WILL WIN ❗ ️. YOU WILL DO ALL OF THAT 💯 , YOU UNDERSTAND ⁉ ️ YOU WILL BECOME A BILLIONAIRE 💵 📈 AND REBUILD YOUR FUCKING LIFE 🤯`,
			`🦍 🗣️ DO NOT GET IT TWISTED 🌪️ , DO NOT GAMBLE 🚫 . DO NOT START GAMBLING ❌ . GAMBLING IS ENTERTAINMENT 🎰 AND ENTERTAINMENT ONLY 👍 . YOU WONT BREAK EVEN 🛑 , YOU WONT WIN ⚠️ ️. YOU WONT DO ANY OF THAT 💯 , YOU UNDERSTAND ⁉️ ️ YOU WILL ONLY GO INTO DEBT 💵 📉 AND RUIN YOUR FUCKING LIFE 😵`,
		)

		return ctx.Reply(gumi.Text(text))
	}
}

func cake(*bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		eb := embeds.NewBuilder()
		eb.Title("Local God eats cake").
			Description("🙏🍰").
			Image("https://cdn.discordapp.com/attachments/1129829799179853914/1160609012115578970/haruhi-haruhi-suzumiya.gif")

		return ctx.Reply(gumi.Embed(eb.Finalize()))
	}
}

var frenzyImages = []string{
	"https://imgpx.com/en/dPGoDCuvghEO.png",
	"https://imgpx.com/en/jDTFDfxEslG0.png",
	"https://imgpx.com/en/l7Ie7kYbEVmc.png",
	"https://media.tenor.com/wLs85BqnUJUAAAAC/eminence-in-shadow-shadow.gif",
	"https://media.tenor.com/1Smw4gr6c3kAAAAC/eminence-in-shadow-cid-kagenou.gif",
	"https://media.tenor.com/YmcBY9nlAwsAAAAC/cid-kagenou-eminence-in-shadow.gif",
	"https://media.tenor.com/rZGFguPbNkIAAAAC/cid.gif",
	"https://static.wikia.nocookie.net/to-be-a-power-in-the-shadows/images/9/9f/Red_Moon-_Anime.png",
}

func frenzy(*bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		image := arrays.RandomElement(frenzyImages)

		eb := embeds.NewBuilder()
		eb.Title("🔴 The moon is red 🔴").
			Description("*The frenzy has begun.* 🤪\n*We're out of time.* ⏳\n*Run if you value your life.* 🏃").
			Color(0x880808).
			Image(*image)

		return ctx.Reply(gumi.Embed(eb.Finalize()))
	}
}

func kaiki(*bot.Bot) gumi.Handler {
	return func(ctx *gumi.Context) error {
		return ctx.Reply(gumi.Text("https://imgpx.com/en/3EUv19fdWN78.png"))
	}
}
