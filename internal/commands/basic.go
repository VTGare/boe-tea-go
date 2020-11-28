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

	ch, _ := s.UserChannelCreate(utils.AuthorID)
	_, err := s.ChannelMessageSendEmbed(ch.ID, eb.Finalize())
	if err != nil {
		return err
	}

	return nil
}

func invite(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) error {
	_, err := s.ChannelMessageSend(m.ChannelID, "Thanks for inviting me to more places! https://discord.com/api/oauth2/authorize?client_id=636468907049353216&permissions=537259072&scope=bot")
	if err != nil {
		return err
	}

	return nil
}

func support(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) error {
	_, err := s.ChannelMessageSend(m.ChannelID, "**Support server invite link:** https://discord.gg/hcxuHE7")
	if err != nil {
		return err
	}

	return nil
}
