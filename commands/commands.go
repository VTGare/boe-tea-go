package commands

import "github.com/bwmarrin/discordgo"

var (
	Commands = make(map[string]Command)
)

type Command struct {
	Name        string
	Description string
	GuildOnly   bool
	Exec        func(*discordgo.Session, *discordgo.MessageCreate, []string) error
}
