package commands

import (
	"context"
	"strconv"
	"time"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
)

func init() {
	Commands["migrateDB"] = Command{
		Name:            "migrateDB",
		Description:     ".",
		GuildOnly:       false,
		Exec:            migrateDB,
		Help:            false,
		AdvancedCommand: false,
		ExtendedHelp:    nil,
	}
}

func migrateDB(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if m.Author.ID != utils.AuthorID {
		return nil
	}

	c := database.DB.Collection("guildsettings")
	res, err := c.UpdateMany(context.Background(), bson.M{}, bson.M{
		"$set": bson.M{
			"created_at": time.Now(),
			"updated_at": time.Now(),
		},
	})
	if err != nil {
		return err
	}

	s.ChannelMessageSend(m.ChannelID, "Modified: "+strconv.FormatInt(res.ModifiedCount, 10))
	return nil
}
