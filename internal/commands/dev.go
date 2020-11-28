package commands

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/embeds"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
)

func init() {
	dg := Router.AddGroup(&gumi.Group{
		Name: "dev",
	})
	dg.IsVisible = false
	dg.AddCommand(&gumi.Command{
		Name: "update",
		Exec: updateDB,
	})
	dg.AddCommand(&gumi.Command{
		Name: "message",
		Exec: message,
	})
	dg.AddCommand(&gumi.Command{
		Name: "stats",
		Exec: devstats,
	})
}

func message(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if m.Author.ID != utils.AuthorID {
		return nil
	}

	if len(args) == 0 {
		return nil
	}

	for _, g := range s.State.Guilds {
		for _, ch := range g.Channels {
			if (strings.Contains(ch.Name, "general") || strings.Contains(ch.Name, "art") || strings.Contains(ch.Name, "sfw") || strings.Contains(ch.Name, "discussion")) && ch.Type == discordgo.ChannelTypeGuildText {
				s.ChannelMessageSend(ch.ID, strings.Join(args, " "))
				break
			}
		}
	}

	return nil
}

func updateDB(_ *discordgo.Session, m *discordgo.MessageCreate, _ []string) error {
	if m.Author.ID != utils.AuthorID {
		return nil
	}

	return nil
}

func devstats(s *discordgo.Session, m *discordgo.MessageCreate, _ []string) error {
	var (
		mem runtime.MemStats
	)
	runtime.ReadMemStats(&mem)

	guilds := len(s.State.Guilds)
	channels := 0
	for _, g := range s.State.Guilds {
		channels += len(g.Channels)
	}
	latency := s.HeartbeatLatency().Round(1 * time.Millisecond)

	eb := embeds.NewBuilder()
	eb.Title("Bot stats").Thumbnail(utils.DefaultEmbedImage)
	eb.AddField("Guilds", strconv.Itoa(guilds), true).AddField("Channels", strconv.Itoa(channels), true)
	eb.AddField("Latency", latency.String(), true).AddField("Shards", strconv.Itoa(s.ShardCount))
	eb.AddField("RAM used", fmt.Sprintf("%v MB", mem.Alloc/1024/1024))

	s.ChannelMessageSendEmbed(m.ChannelID, eb.Finalize())
	return nil
}
