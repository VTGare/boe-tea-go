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
	"github.com/VTGare/gumi"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
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

func updateDB(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if m.Author.ID != utils.AuthorID {
		return nil
	}

	c := database.DB.UserSettings
	res, err := c.UpdateMany(context.Background(), bson.M{}, bson.M{
		"$set": bson.M{
			"nsfw": true,
		},
	})
	if err != nil {
		return err
	}

	s.ChannelMessageSend(m.ChannelID, "Modified: "+strconv.FormatInt(res.ModifiedCount, 10))
	return nil
}

func devstats(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
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
				Inline: false,
			},
			{Name: "RAM used", Value: fmt.Sprintf("%v MB", mem.Alloc/1024/1024), Inline: false},
		},
	})
	return nil
}
