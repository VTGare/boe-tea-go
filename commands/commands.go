package commands

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

var (
	//CommandGroups stores groups of commands.
	CommandGroups = make(map[string]CommandGroup)
)

//Command is a structure that defines cmmmand behaviour.
type Command struct {
	Name        string
	Aliases     []string
	Description string
	GuildOnly   bool
	Exec        func(*discordgo.Session, *discordgo.MessageCreate, []string) error
	Help        *HelpSettings
}

//CommandGroup is a structure that groups similar commands.
type CommandGroup struct {
	Name        string
	Description string
	NSFW        bool
	Commands    map[string]Command
	IsVisible   bool
}

//HelpSettings defines Command's behaviour if help command was executed on it.
type HelpSettings struct {
	IsVisible    bool
	ExtendedHelp []*discordgo.MessageEmbedField
}

func (g *CommandGroup) addCommand(c *Command) {
	if g.Commands == nil {
		g.Commands = make(map[string]Command)
	}

	g.Commands[c.Name] = *c
	for _, alias := range c.Aliases {
		g.Commands[alias] = *c
	}
}

func newCommand(name, description string) *Command {
	return &Command{
		Name:        name,
		Aliases:     make([]string, 0),
		Description: description,
		GuildOnly:   false,
		Exec:        nil,
		Help: &HelpSettings{
			IsVisible:    true,
			ExtendedHelp: nil,
		},
	}
}

func (c *Command) setName(name string) *Command {
	c.Name = name
	return c
}

func (c *Command) setDescription(desc string) *Command {
	c.Description = desc
	return c
}

func (c *Command) setAliases(aliases ...string) *Command {
	c.Aliases = aliases
	return c
}

func (c *Command) setGuildOnly(guildOnly bool) *Command {
	c.GuildOnly = guildOnly
	return c
}

func (c *Command) setExec(exec func(*discordgo.Session, *discordgo.MessageCreate, []string) error) *Command {
	c.Exec = exec
	return c
}

func (c *Command) setHelp(help *HelpSettings) *Command {
	c.Help = help
	return c
}

func (c *Command) createHelp() string {
	str := ""
	if len(c.Aliases) != 0 {
		str += fmt.Sprintf("**Aliases:** %v\n", strings.Join(c.Aliases, ", "))
	}
	str += c.Description

	return str
}
