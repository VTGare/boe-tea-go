package commands

import (
	"context"
	"strconv"
	"strings"

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
			if strings.Contains(ch.Name, "general") && ch.Type == discordgo.ChannelTypeGuildText {
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

	c := database.DB.Collection("guildsettings")
	res, err := c.UpdateMany(context.Background(), bson.M{}, bson.M{
		"$set": bson.M{
			"repost": "enabled",
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
