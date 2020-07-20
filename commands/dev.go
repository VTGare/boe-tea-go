package commands

import (
	"context"
	"strconv"

	"github.com/VTGare/boe-tea-go/database"
	"github.com/VTGare/boe-tea-go/utils"
	"github.com/bwmarrin/discordgo"
	"go.mongodb.org/mongo-driver/bson"
)

func init() {
	devGroup := CommandGroup{
		Name:        "dev",
		Description: "-",
		NSFW:        false,
		Commands:    make(map[string]Command),
		IsVisible:   false,
	}

	migrateCommand := newCommand("migrate", "-").setExec(migrateDB)
	testCommand := newCommand("test", "-").setExec(testCmd)

	devGroup.addCommand(migrateCommand)
	devGroup.addCommand(testCommand)
	CommandGroups["dev"] = devGroup
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

func testCmd(s *discordgo.Session, m *discordgo.MessageCreate, args []string) error {
	if m.Author.ID != utils.AuthorID {
		return nil
	}

	embed1 := discordgo.MessageEmbed{
		Title:       "[named links](https://discordapp.com)",
		Description: "[named links](https://discordapp.com)",
	}

	s.ChannelMessageSendEmbed(m.ChannelID, &embed1)
	return nil
}
