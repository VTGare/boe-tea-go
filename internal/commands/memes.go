package commands

import (
	"bytes"
	"text/template"

	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
)

var (
	nuggetsTemplate = template.Must(template.New("nuggets").Parse(`>{{.Ryo}} and {{.Amelia}} sits side by side watching a movie
>They're eating pack of chicken nuggets while watching
>Somehow the takeout didn't include much bbq sauce except for a half filled one
>Sauce has been depleted after {{.Ryo}} has eaten 3 nuggets
>{{.Ryo}}: Watson, we're out of sauce. Do you have any in your fridge?
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
	memes := Router.AddGroup(&gumi.Group{
		Name: "memes",
	})
	memes.IsVisible = false

	memes.AddCommand(&gumi.Command{
		Name: "borgar",
		Exec: borgar,
	})
	memes.AddCommand(&gumi.Command{
		Name: "brainpower",
		Exec: brainpower,
	})
	memes.AddCommand(&gumi.Command{
		Name: "minesweeper",
		Exec: minesweeper,
	})
	memes.AddCommand(&gumi.Command{
		Name: "nuggets",
		Exec: nuggets,
	})
}

func borgar(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:     "🦕🍔",
		Timestamp: utils.EmbedTimestamp(),
		Color:     utils.EmbedColor,
		Image: &discordgo.MessageEmbedImage{
			URL: "https://images-ext-2.discordapp.net/external/gRgdT4gZIPbY26qK9iM0edWQA4hYPZF5RvxVdSeXhRQ/https/i.kym-cdn.com/photos/images/original/001/568/282/ef2.gif?width=438&height=444",
		},
	})
	return nil
}

type NuggetsCopypasta struct {
	Amelia string
	Ryo    string
}

func nuggets(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) < 2 {
		return utils.ErrNotEnoughArguments
	}

	n := &NuggetsCopypasta{Amelia: args[0], Ryo: args[1]}

	buf := new(bytes.Buffer)
	nuggetsTemplate.Execute(buf, n)

	s.ChannelMessageSend(m.ChannelID, buf.String())
	return nil
}

func brainpower(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	s.ChannelMessageSend(m.ChannelID, "O-oooooooooo AAAAE-A-A-I-A-U- JO-oooooooooooo AAE-O-A-A-U-U-A- E-eee-ee-eee AAAAE-A-E-I-E-A-JO-ooo-oo-oo-oo EEEEO-A-AAA-AAAA")
	return nil
}

func minesweeper(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	ms := &Minesweeper{}
	ms.generateField()

	s.ChannelMessageSend(m.ChannelID, ms.String())
	return nil
}
