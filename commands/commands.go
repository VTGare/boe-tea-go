package commands

import (
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
)

type ExecFunc func(*bot.Bot, *state.State) (api.InteractionResponse, error)

type Command struct {
	CreateData  api.CreateCommandData
	Exec        ExecFunc
	Subcommands map[string]ExecFunc
}

/*

{
		CreateData: api.CreateCommandData{
			Name:        "artchannels",
			Description: "Manipulation with server's art channels.",
			Options: discord.CommandOptions{
				discord.NewSubcommandOption("list", "List art channels."),
				discord.NewSubcommandOption("add", "Add art channels."),
				discord.NewSubcommandOption("remove", "Remove art channels."),
			},
			Type: discord.ChatInputCommand,
		},
},

*/

var commands = map[string]*Command{
	"artchannels": cmd("artchannels", "Manipulation with server's art channels.", discord.ChatInputCommand).
		withSubcommand("list", "List art channels", nil).
		withSubcommand("add", "Add an art channel", nil, discord.NewChannelOption("channel", "Art channel.", true)).
		withSubcommand("remove", "Remove an art channel", nil, discord.NewChannelOption("channel", "Art channel", true)),

	"ping": {
		CreateData: api.CreateCommandData{
			Name:        "ping",
			Description: "Pong!",
			Type:        discord.ChatInputCommand,
		},
		Exec: ping,
	},

	"about": {
		CreateData: api.CreateCommandData{
			Name:        "about",
			Description: "A bunch of useful links.",
			Type:        discord.ChatInputCommand,
		},
		Exec: ping,
	},

	"stats": {
		CreateData: api.CreateCommandData{
			Name:        "stats",
			Description: "Shows runtime stats.",
			Type:        discord.ChatInputCommand,
		},
		Exec: ping,
	},

	"": {
		CreateData: api.CreateCommandData{
			Name:        "ping",
			Description: "Shows bot's response time.",
			Type:        discord.ChatInputCommand,
		},
		Exec: ping,
	},
}

func All() map[string]*Command {
	return commands
}

func CreateData() []api.CreateCommandData {
	cmds := make([]api.CreateCommandData, 0, len(commands))
	for _, command := range commands {
		cmds = append(cmds, command.CreateData)
	}

	return cmds
}

func Find(name string) (*Command, bool) {
	cmd, ok := commands[name]
	return cmd, ok
}

func cmd(name, desc string, t discord.CommandType) *Command {
	return &Command{
		CreateData: api.CreateCommandData{
			Name:        name,
			Description: desc,
			Type:        t,
			Options:     discord.CommandOptions{},
		},
	}
}

func (c *Command) withExec(exec ExecFunc) *Command {
	c.Exec = exec
	return c
}

func (c *Command) withSubcommand(name, desc string, exec ExecFunc, opts ...discord.CommandOptionValue) *Command {
	c.CreateData.Options = append(c.CreateData.Options, discord.NewSubcommandOption(name, desc, opts...))
	c.Subcommands[name] = exec
	return c
}

func (c *Command) withOptions(opts ...discord.CommandOption) *Command {
	c.CreateData.Options = append(c.CreateData.Options, opts...)
	return c
}
