package commands

import (
	"context"

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
	c.DeleteMany(context.Background(), bson.M{})
	database.GuildCache = make(map[string]database.GuildSettings)
	utils.CreateDB(s.State.Guilds)
	return nil
}
