package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
)

func ping(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) error {
	_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(":ping_pong: Pong! Latency: ***%v***", s.HeartbeatLatency().Round(1*time.Millisecond)))
	if err != nil {
		return err
	}
	return nil
}

func about(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) error {
	eb := embeds.NewBuilder()
	eb.Title("ℹ About").Thumbnail(s.State.User.AvatarURL(""))
	eb.Description(
		`Boe Tea is a Swiss Army Knife of art sharing and moderation on Discord.
If you want to copy an invite link, simply right click it and press Copy Link.

***Special thanks to our patron(s):***
- Nom (Indy#4649) | 4 months
`)
	eb.AddField("Support server", "[Click here desu~](https://discord.gg/hcxuHE7)", true)
	eb.AddField("Invite link", "[Click here desu~](https://discord.com/api/oauth2/authorize?client_id=636468907049353216&permissions=537259072&scope=bot)", true)
	eb.AddField("Patreon", "[Click here desu~](https://patreon.com/vtgare)", true)

	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	return nil
}

func feedback(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	message := strings.Join(args, " ")
	eb := embeds.NewBuilder()
	eb.Title(fmt.Sprintf("Feedback from %v", m.Author.String())).Thumbnail(m.Author.AvatarURL("")).Description(message)
	if len(m.Attachments) >= 1 {
		eb.Image(m.Attachments[0].URL)
	}

	ch, err := s.UserChannelCreate(utils.AuthorID)
	if err != nil {
		return err
	}

	_, err = s.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
	if err != nil {
		return err
	}

	eb.Clear()
	s.ChannelMessageSendEmbed(m.ChannelID, eb.SuccessTemplate("Feedback message has been sent.").Finalize())
	return nil
}
