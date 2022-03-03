package commands

import (
	"github.com/VTGare/boe-tea-go/pkg/bot"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
)

type Command struct {
	CreateData api.CreateCommandData
	Exec       func(*bot.Bot, *state.State) (api.InteractionResponse, error)
}

var commands = map[string]Command{
	"ping": {
		CreateData: api.CreateCommandData{
			Name:        "ping",
			Description: "Shows bot's response time.",
			Type:        discord.ChatInputCommand,
		},
		Exec: ping,
	},
}

func All() map[string]Command {
	return commands
}

func CreateData() []api.CreateCommandData {
	cmds := make([]api.CreateCommandData, 0, len(commands))
	for _, command := range commands {
		cmds = append(cmds, command.CreateData)
	}

	return cmds
}

func Find(name string) (Command, bool) {
	cmd, ok := commands[name]
	return cmd, ok
}
