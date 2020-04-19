package commands

import "github.com/bwmarrin/discordgo"

var (
	//Commands stores bot commands
	Commands = make(map[string]Command)
)

//Command is a structure that defines cmmmand behaviour.
type Command struct {
	Name        string
	Description string
	GuildOnly   bool
	Exec        func(*discordgo.Session, *discordgo.MessageCreate, []string) error
}
