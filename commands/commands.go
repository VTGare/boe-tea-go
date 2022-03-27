package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
	"go.uber.org/atomic"
)

type ExecFunc func(context.Context, *bot.Bot, *state.State, discord.InteractionEvent) (api.InteractionResponse, error)

type Command struct {
	GuildOnly   bool
	AuthorOnly  bool
	Modal       bool
	CreateData  api.CreateCommandData
	Exec        ExecFunc
	Subcommands map[string]ExecFunc
}

type commandOption func(*Command)

var commands = map[string]*Command{
	// General commands
	"artchannels": group("artchannels", "Manipulation with server's art channels.", guildOnly).
		subcommand("list", "List art channels", listArtChannels).
		subcommand("add", "Add art channels", addArtChannels, discord.NewChannelOption("channel", "Art channel. If category, every channel is added.", true)).
		subcommand("remove", "Remove art channels", removeArtChannels, discord.NewChannelOption("channel", "Art channel. If category, every channel is removed.", true)),

	"settings": group("set", "Show or change server settings.", guildOnly).
		subcommand("show", "Show server settings", showSettings).
		subcommand("prefix", "Change command prefix.",
			changeSetting(func(g *store.Guild, opt discord.CommandInteractionOptions) (interface{}, error) {
				old := g.Prefix
				new := opt[0].String()
				if len(new) > 5 {
					return nil, errPrefixTooLong
				}

				g.Prefix = new

				return old, nil
			}), discord.NewStringOption("prefix", "Regular command prefix", true),
		).
		subcommand("nsfw", "Change NSFW setting.",
			changeSetting(func(g *store.Guild, opt discord.CommandInteractionOptions) (interface{}, error) {
				old := g.NSFW

				new, err := opt[0].BoolValue()
				if err != nil {
					return nil, err
				}

				g.NSFW = new
				return old, nil
			}), discord.NewBooleanOption("enabled", "Switch NSFW setting.", true),
		).
		subcommand("pixiv", "Change Pixiv embedding setting.",
			changeSetting(func(g *store.Guild, opt discord.CommandInteractionOptions) (interface{}, error) {
				old := g.Pixiv

				new, err := opt[0].BoolValue()
				if err != nil {
					return nil, err
				}

				g.Pixiv = new
				return old, nil
			}), discord.NewBooleanOption("enabled", "Switch Pixiv embedding", true),
		).
		subcommand("twitter", "Change Twitter embedding setting.",
			changeSetting(func(g *store.Guild, opt discord.CommandInteractionOptions) (interface{}, error) {
				old := g.Twitter

				new, err := opt[0].BoolValue()
				if err != nil {
					return nil, err
				}

				g.Twitter = new
				return old, nil
			}), discord.NewBooleanOption("enabled", "Switch Twitter embedding.", true),
		).
		subcommand("deviant", "Change DeviantArt embedding setting.",
			changeSetting(func(g *store.Guild, opt discord.CommandInteractionOptions) (interface{}, error) {
				old := g.Deviant

				new, err := opt[0].BoolValue()
				if err != nil {
					return nil, err
				}

				g.Deviant = new
				return old, nil
			}), discord.NewBooleanOption("enabled", "Switch DeviantArt embedding", true),
		).
		subcommand("artstation", "Change ArtStation embedding setting.",
			changeSetting(func(g *store.Guild, opt discord.CommandInteractionOptions) (interface{}, error) {
				old := g.Artstation

				new, err := opt[0].BoolValue()
				if err != nil {
					return nil, err
				}

				g.Artstation = new
				return old, nil
			}), discord.NewBooleanOption("enabled", "Switch ArtStation embedding.", true),
		).
		subcommand("crosspost", "Change crosspost setting.",
			changeSetting(func(g *store.Guild, opt discord.CommandInteractionOptions) (interface{}, error) {
				old := g.Crosspost

				new, err := opt[0].BoolValue()
				if err != nil {
					return nil, err
				}

				g.Crosspost = new
				return old, nil
			}), discord.NewBooleanOption("enabled", "Switch crosspost setting.", true),
		).
		subcommand("reactions", "Change message reactions setting.",
			changeSetting(func(g *store.Guild, opt discord.CommandInteractionOptions) (interface{}, error) {
				old := g.Reactions

				new, err := opt[0].BoolValue()
				if err != nil {
					return nil, err
				}

				g.Reactions = new
				return old, nil
			}), discord.NewBooleanOption("enabled", "Switch message reactions.", true),
		).
		subcommand("tags", "Change embed tags setting.",
			changeSetting(func(g *store.Guild, opt discord.CommandInteractionOptions) (interface{}, error) {
				old := g.Tags

				new, err := opt[0].BoolValue()
				if err != nil {
					return nil, err
				}

				g.Tags = new
				return old, nil
			}), discord.NewBooleanOption("enabled", "Switch embed tags", true),
		).
		subcommand("footer", "Change embed footer setting.",
			changeSetting(func(g *store.Guild, opt discord.CommandInteractionOptions) (interface{}, error) {
				old := g.FlavourText

				new, err := opt[0].BoolValue()
				if err != nil {
					return nil, err
				}

				g.FlavourText = new
				return old, nil
			}), discord.NewBooleanOption("enabled", "Switch embed footer.", true),
		).
		subcommand("repost", "Change repost detection setting.",
			changeSetting(func(g *store.Guild, opt discord.CommandInteractionOptions) (interface{}, error) {
				old := g.Repost

				detection, err := opt[0].BoolValue()
				if err != nil {
					return nil, err
				}

				if !detection {
					g.Repost = "disabled"
					return old, nil
				}

				var strict bool
				if len(opt) == 2 {
					strict, err = opt[1].BoolValue()
					if err != nil {
						return nil, err
					}
				}

				if strict {
					g.Repost = "strict"
				} else {
					g.Repost = "enabled"
				}
				return old, nil
			}), discord.NewBooleanOption("enabled", "Switch repost detection", true), discord.NewBooleanOption("strict", "Delete reposts", false),
		).
		subcommand("expiration", "Change repost expiration duration.",
			changeSetting(func(g *store.Guild, opt discord.CommandInteractionOptions) (interface{}, error) {
				old := g.RepostExpiration
				new := opt[0].String()

				dur, err := time.ParseDuration(new)
				if err != nil {
					return nil, fmt.Errorf("failed to parse duration: %w", err)
				}

				g.RepostExpiration = dur
				return old, nil
			}), discord.NewStringOption("duration", "Format: [integer]m/h. Example: 12m is 12 minutes.", true),
		).
		subcommand("limit", "Change album size limit setting.",
			changeSetting(func(g *store.Guild, opt discord.CommandInteractionOptions) (interface{}, error) {
				old := g.Limit

				new, err := opt[0].IntValue()
				if err != nil {
					return nil, err
				}

				g.Limit = int(new)
				return old, nil
			}), discord.NewIntegerOption("limit", "New album size limit", true),
		),

	"stats": group("stats", "A bunch of interesting global Boe Tea stats").
		subcommand("runtime", "General runtime stats.", runtimeStats).
		subcommand("artworks", "Artwork provider stats.", artworkStats).
		subcommand("commands", "Command usage stats.", commandStats),

	"ping":     cmd("ping", "Pong!", discord.ChatInputCommand, ping),
	"about":    cmd("about", "A bunch of useful links.", discord.ChatInputCommand, about),
	"feedback": cmd("feedback", "Send an angry message to the dev.", discord.ChatInputCommand, feedback, modal),
	"reply": cmd("reply", "Dev's way to reply to your angry message.", discord.ChatInputCommand, reply, authorOnly, modal).
		withOptions(discord.NewUserOption("recipient", "Reply receiver", true)),
}

var Hits = make(map[string]*atomic.Int64)

func init() {
	for name := range commands {
		Hits[name] = atomic.NewInt64(0)
	}
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

func cmd(name, desc string, t discord.CommandType, exec ExecFunc, opts ...commandOption) *Command {
	c := &Command{
		CreateData: api.CreateCommandData{
			Name:        name,
			Description: desc,
			Type:        t,
			Options:     discord.CommandOptions{},
		},
		Exec: exec,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func group(name, desc string, opts ...commandOption) *Command {
	c := &Command{
		CreateData: api.CreateCommandData{
			Name:        name,
			Description: desc,
			Type:        discord.ChatInputCommand,
			Options:     discord.CommandOptions{},
		},
		Subcommands: make(map[string]ExecFunc),
	}

	for _, opt := range opts {
		opt(c)
	}

	c.Exec = func(ctx context.Context, b *bot.Bot, s *state.State, e discord.InteractionEvent) (api.InteractionResponse, error) {
		ci := e.Data.(*discord.CommandInteraction)
		subcommand := ci.Options[0]
		return c.Subcommands[subcommand.Name](ctx, b, s, e)
	}

	return c
}

func (c *Command) subcommand(name, desc string, exec ExecFunc, opts ...discord.CommandOptionValue) *Command {
	c.CreateData.Options = append(c.CreateData.Options, discord.NewSubcommandOption(name, desc, opts...))
	c.Subcommands[name] = exec
	return c
}

func (c *Command) withOptions(opts ...discord.CommandOption) *Command {
	c.CreateData.Options = append(c.CreateData.Options, opts...)
	return c
}

func guildOnly(c *Command) {
	c.GuildOnly = true
}

func authorOnly(c *Command) {
	c.AuthorOnly = true
}

func modal(c *Command) {
	c.Modal = true
}
