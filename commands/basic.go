package commands

import (
	"fmt"
	"log"
	"time"

	"github.com/VTGare/boe-tea-go/services"
	"github.com/bwmarrin/discordgo"
)

func init() {
	Commands["ping"] = Command{
		Name:        "ping",
		Description: "Pong",
		GuildOnly:   false,
		Exec:        ping,
	}
	Commands["test"] = Command{
		Name:        "test",
		Description: "Pong",
		GuildOnly:   false,
		Exec:        test,
	}
}

func ping(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	_, err := s.ChannelMessageSend(m.ChannelID, fmt.Sprintf(":ping_pong: Pong! Latency: ***%v***", s.HeartbeatLatency().Round(1*time.Millisecond)))
	if err != nil {
		return err
	}
	return nil
}

func help(s *discordgo.Session, m *discordgo.MessageCreate) error {
	return nil
}

func test(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if m.Author.ID != "244208152776540160" {
		return nil
	}

	res, err := services.SearchSauceByURL("https://images-ext-1.discordapp.net/external/lPAq5wxKWxDNO358Ea9fDrjBjfW5Kl02BuoFEE8mrZY/https/pbs.twimg.com/media/EVy0c0CVAAAeEgb.jpg%3Alarge?width=291&height=441")
	if err != nil {
		log.Println(err)
	}

	log.Println(res.Header.ResultsReturned)

	return nil
}
