package commands

import (
	"context"
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/database"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
)

func init() {
	dg := CommandFramework.AddGroup("dev")
	dg.IsVisible = false
	dg.AddCommand("migrate", migrateDB)
	dg.AddCommand("test", test)
	dg.AddCommand("message", message)
	dg.AddCommand("devstats", devstats)
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

func migrateDB(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if m.Author.ID != utils.AuthorID {
		return nil
	}

	c := database.DB.GuildSettings
	res, err := c.UpdateMany(context.Background(), bson.M{"repost": "enabled"}, bson.M{
		"$set": bson.M{
			"repost":  "disabled",
			"twitter": false,
		},
	})
	if err != nil {
		return err
	}

	s.ChannelMessageSend(m.ChannelID, "Modified: "+strconv.FormatInt(res.ModifiedCount, 10))
	return nil
}

func test(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if m.Author.ID != utils.AuthorID {
		return nil
	}

	if len(args) == 0 {
		return utils.ErrNotEnoughArguments
	}

	return nil
}

func devstats(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if m.Author.ID != utils.AuthorID {
		return nil
	}

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

	s.ChannelMessageSendEmbed(m.ChannelID, &discordgo.MessageEmbed{
		Title:     "Bot stats",
		Color:     utils.EmbedColor,
		Timestamp: utils.EmbedTimestamp(),
		Thumbnail: &discordgo.MessageEmbedThumbnail{
			URL: utils.DefaultEmbedImage,
		},
		Fields: []*discordgo.MessageEmbedField{
			{
				Name:   "Guilds",
				Value:  strconv.Itoa(guilds),
				Inline: true,
			},
			{
				Name:   "Channels",
				Value:  strconv.Itoa(channels),
				Inline: true,
			},
			{
				Name:   "Latency",
				Value:  latency.String(),
				Inline: true,
			},
			{
				Name:   "Shards",
				Value:  strconv.Itoa(s.ShardCount),
				Inline: true,
			},
			{Name: "RAM used", Value: fmt.Sprintf("%v MB", mem.Alloc/1024/1024), Inline: true},
		},
	})
	return nil
}
