package commands

import (
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
)

func ping(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
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
	embed := &discordgo.MessageEmbed{
		Title: fmt.Sprintf("Feedback from %v", m.Author.String()),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: m.Author.AvatarURL(""),
		},
		Description: message,
		Timestamp:   utils.EmbedTimestamp(),
		Color:       utils.EmbedColor,
	}

	if len(m.Attachments) >= 1 {
		embed.Image = &discordgo.MessageEmbedImage{
			URL: m.Attachments[0].URL,
		}
	}

	ch, _ := s.UserChannelCreate(utils.AuthorID)
	_, err := s.ChannelMessageSendEmbed(ch.ID, embed)
	if err != nil {
		return err
	}

	return nil
}

func invite(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	_, err := s.ChannelMessageSend(m.ChannelID, "**Here's my invitation link, spread the word:** https://discordapp.com/api/oauth2/authorize?client_id=636468907049353216&permissions=537250880&scope=bot")
	if err != nil {
		return err
	}

	return nil
}

func support(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	_, err := s.ChannelMessageSend(m.ChannelID, "**Support server invite link:** https://discord.gg/hcxuHE7")
	if err != nil {
		return err
	}

	return nil
}
