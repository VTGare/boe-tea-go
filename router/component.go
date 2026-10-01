package router

import (
	"strings"

	"github.com/bwmarrin/discordgo"
)

// ComponentHandler handles clicks, selects and modal submits on components
// whose custom IDs came from ComponentID.
type ComponentHandler func(ctx *ComponentContext) error

// componentPrefix namespaces the router's component custom IDs.
const componentPrefix = "rt:"

// maxCustomID is Discord's custom ID limit.
const maxCustomID = 100

// ComponentID builds a custom ID that routes back to cmd's Components
// handler with args. It returns "" when the ID would exceed Discord's
// 100-character limit; args must not contain ':'.
func ComponentID(cmd *Command, args ...string) string {
	id := componentPrefix + cmd.Root().Name
	if len(args) > 0 {
		id += ":" + strings.Join(args, ":")
	}

	if len(id) > maxCustomID {
		return ""
	}

	return id
}

// ComponentContext is a button click, select or modal submit.
type ComponentContext struct {
	Session     *discordgo.Session
	Router      *Router
	Command     *Command
	Interaction *discordgo.InteractionCreate
	// Args are the custom ID's parts after the command name.
	Args []string

	responded bool
}

// Arg returns the i-th arg, or "" when absent.
func (c *ComponentContext) Arg(i int) string {
	if i < len(c.Args) {
		return c.Args[i]
	}
	return ""
}

func (c *ComponentContext) GuildID() string { return c.Interaction.GuildID }

func (c *ComponentContext) ChannelID() string { return c.Interaction.ChannelID }

// UserID returns the ID of the user who clicked.
func (c *ComponentContext) UserID() string {
	if m := c.Interaction.Member; m != nil && m.User != nil {
		return m.User.ID
	}
	if c.Interaction.User != nil {
		return c.Interaction.User.ID
	}
	return ""
}

// Permissions are the user's channel permissions (all of them in DMs).
func (c *ComponentContext) Permissions() int64 {
	if c.Interaction.Member == nil {
		return discordgo.PermissionAll
	}
	return c.Interaction.Member.Permissions
}

// IsModal reports whether this is a modal submit.
func (c *ComponentContext) IsModal() bool {
	return c.Interaction.Type == discordgo.InteractionModalSubmit
}

// Values are a select menu's chosen values.
func (c *ComponentContext) Values() []string {
	if c.IsModal() {
		return nil
	}
	return c.Interaction.MessageComponentData().Values
}

// ResolvedChannels are the channels picked in a channel select, by ID.
func (c *ComponentContext) ResolvedChannels() map[string]*discordgo.Channel {
	if c.IsModal() {
		return nil
	}
	return c.Interaction.MessageComponentData().Resolved.Channels
}

// TextInput is a modal text input's submitted value, or "".
func (c *ComponentContext) TextInput(customID string) string {
	if !c.IsModal() {
		return ""
	}

	for _, row := range c.Interaction.ModalSubmitData().Components {
		r, ok := row.(*discordgo.ActionsRow)
		if !ok {
			continue
		}

		for _, comp := range r.Components {
			if in, ok := comp.(*discordgo.TextInput); ok && in.CustomID == customID {
				return in.Value
			}
		}
	}

	return ""
}

// Update edits the message the component is on.
func (c *ComponentContext) Update(r *Response) error {
	return c.respond(discordgo.InteractionResponseUpdateMessage, &discordgo.InteractionResponseData{
		Content:         r.Content,
		Embeds:          r.Embeds,
		Components:      r.Components,
		AllowedMentions: r.AllowedMentions,
	})
}

// Reply sends a new message; set Ephemeral for an invoker-only one.
func (c *ComponentContext) Reply(r *Response) error {
	return c.respond(discordgo.InteractionResponseChannelMessageWithSource, r.interactionData(false))
}

// Modal opens a modal whose submit routes back with the given custom ID.
func (c *ComponentContext) Modal(customID, title string, inputs ...discordgo.TextInput) error {
	rows := make([]discordgo.MessageComponent, 0, len(inputs))
	for _, in := range inputs {
		rows = append(rows, discordgo.ActionsRow{Components: []discordgo.MessageComponent{in}})
	}

	return c.respond(discordgo.InteractionResponseModal, &discordgo.InteractionResponseData{
		CustomID:   customID,
		Title:      title,
		Components: rows,
	})
}

func (c *ComponentContext) respond(t discordgo.InteractionResponseType, data *discordgo.InteractionResponseData) error {
	if c.responded {
		return ErrAlreadyResponded
	}

	err := c.Session.InteractionRespond(c.Interaction.Interaction, &discordgo.InteractionResponse{Type: t, Data: data})
	if err == nil {
		c.responded = true
	}

	return err
}

func componentCustomID(i *discordgo.InteractionCreate) string {
	if i.Type == discordgo.InteractionModalSubmit {
		return i.ModalSubmitData().CustomID
	}
	return i.MessageComponentData().CustomID
}

// handleComponent routes a component or modal submit to the root command
// named in its custom ID. Components skip middleware, so panics are
// recovered here and errors answered privately.
func (r *Router) handleComponent(s *discordgo.Session, i *discordgo.InteractionCreate) {
	rest, ok := strings.CutPrefix(componentCustomID(i), componentPrefix)
	if !ok {
		return
	}

	name, args, _ := strings.Cut(rest, ":")

	var cmd *Command
	for _, c := range r.Commands() {
		if c.Name == name && c.Components != nil {
			cmd = c
			break
		}
	}
	if cmd == nil {
		return
	}

	ctx := &ComponentContext{Session: s, Router: r, Command: cmd, Interaction: i}
	if args != "" {
		ctx.Args = strings.Split(args, ":")
	}

	defer func() { _ = recover() }()

	if err := cmd.Components(ctx); err != nil && !ctx.responded {
		msg, ok := UserMessageOf(err)
		if !ok {
			msg = "Something went wrong."
		}
		_ = ctx.Reply(Text(msg).Private())
	}
}
