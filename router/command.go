package router

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/bwmarrin/discordgo"
)

// CommandType mirrors Discord application command types.
type CommandType int

const (
	ChatInput CommandType = iota
	// MessageContext runs on a message: right-click it, or reply to it
	// (or pass a link / ID) over prefix.
	MessageContext
	// UserContext runs on a user: right-click them, or pass a mention / ID
	// over prefix (defaults to the author).
	UserContext
)

func (t CommandType) String() string {
	switch t {
	case ChatInput:
		return "chat_input"
	case MessageContext:
		return "message"
	case UserContext:
		return "user"
	}
	return "unknown"
}

func (t CommandType) discord() discordgo.ApplicationCommandType {
	switch t {
	case MessageContext:
		return discordgo.MessageApplicationCommand
	case UserContext:
		return discordgo.UserApplicationCommand
	default:
		return discordgo.ChatApplicationCommand
	}
}

// commandTypeOf is the inverse of CommandType.discord.
func commandTypeOf(t discordgo.ApplicationCommandType) CommandType {
	switch t {
	case discordgo.MessageApplicationCommand:
		return MessageContext
	case discordgo.UserApplicationCommand:
		return UserContext
	default:
		return ChatInput
	}
}

type Handler func(ctx *Context) error

// Middleware wraps a Handler. Router middleware runs first, then the
// command chain's, outermost first.
type Middleware func(next Handler) Handler

// Check gates a command: it runs after middleware, before cooldowns.
// Reject with a UserFacing error to tell the user why.
type Check func(ctx *Context) error

// Command is declared once and works as both a slash command and a
// prefix command.
type Command struct {
	// Chat input names: 1-32 lowercase chars, no spaces. Context menu
	// names are free-form.
	Name string
	// Shown in Discord and in help (1-100 chars). Discord ignores it for
	// context menu commands.
	Description string
	// Defaults to ChatInput.
	Type     CommandType
	Category string
	// Extra prefix names; slash ignores them.
	Aliases []string
	// Parsed positionally in declaration order over prefix.
	Options []*Option
	// Subcommands turn this into a group: no options or handler allowed,
	// at most two levels deep (command > group > sub).
	Subcommands []*Command
	// Required for non-group commands.
	Handler Handler
	// Group checks also guard its subcommands.
	Checks     []Check
	Middleware []Middleware
	// Set on a group to cover its subcommands too.
	Cooldown *Cooldown
	// Acknowledge first ("thinking…", or typing over prefix), giving the
	// handler up to 15 minutes.
	Defer bool
	// Invoker-only replies. No effect over prefix.
	Ephemeral bool
	// Age-restricts the slash command. This alone changes nothing at runtime;
	// add NSFW to Checks to gate prefix invocations too.
	NSFW bool
	// Hides the slash command by default (admins can override). Also not a
	// runtime check; use HasPermissions for that.
	DefaultMemberPermissions *int64
	// Where the slash command shows up; empty means Discord's default.
	Contexts         []discordgo.InteractionContextType
	IntegrationTypes []discordgo.ApplicationIntegrationType
	// Register the slash command only here instead of globally.
	GuildIDs      []string
	DisableSlash  bool
	DisablePrefix bool
	Hidden        bool
	// Shown in help; write without prefix, e.g. "set prefix !".
	Examples []string

	// Components handles interactions on components built with
	// ComponentID. Only root commands receive them.
	Components ComponentHandler

	parent *Command
}

// Discord's application command limits.
const (
	maxNameLength        = 32
	maxDescriptionLength = 100
	// maxEntries caps options, subcommands and choices alike.
	maxEntries = 25
)

// chatNameRe is Discord's chat input naming rule, 1-maxNameLength characters.
var chatNameRe = regexp.MustCompile(`^[-_\p{L}\p{N}]{1,32}$`)

// validChatName reports whether s fits Discord's slash-command naming.
func validChatName(s string) bool {
	return chatNameRe.MatchString(s) && s == strings.ToLower(s)
}

// validDescription reports whether s fits Discord's 1-100 character
// description limits.
func validDescription(s string) bool {
	n := utf8.RuneCountInString(s)
	return n >= 1 && n <= maxDescriptionLength
}

func (c *Command) Parent() *Command { return c.parent }

func (c *Command) Root() *Command {
	cur := c
	for cur.parent != nil {
		cur = cur.parent
	}
	return cur
}

func (c *Command) Path() []string {
	chain := c.chain()
	path := make([]string, len(chain))
	for i, cmd := range chain {
		path[i] = cmd.Name
	}
	return path
}

func (c *Command) QualifiedName() string { return strings.Join(c.Path(), " ") }

func (c *Command) IsGroup() bool { return len(c.Subcommands) > 0 }

// Subcommand finds a direct subcommand by name or alias, ignoring case.
func (c *Command) Subcommand(name string) *Command {
	for _, sub := range c.Subcommands {
		if strings.EqualFold(sub.Name, name) {
			return sub
		}
		for _, alias := range sub.Aliases {
			if strings.EqualFold(alias, name) {
				return sub
			}
		}
	}
	return nil
}

func (c *Command) Walk(fn func(*Command)) {
	fn(c)
	for _, sub := range c.Subcommands {
		sub.Walk(fn)
	}
}

// Usage renders e.g. "/set prefix <value>" for lead "/",
// "bt!set prefix <value>" for lead "bt!".
func (c *Command) Usage(lead string) string {
	switch c.Type {
	case MessageContext:
		if lead == "/" {
			return fmt.Sprintf("Right-click a message → Apps → %s", c.Name)
		}
		return fmt.Sprintf("%s%s (while replying to a message, or with a message link)", lead, c.prefixName())
	case UserContext:
		if lead == "/" {
			return fmt.Sprintf("Right-click a user → Apps → %s", c.Name)
		}
		return fmt.Sprintf("%s%s [user]", lead, c.prefixName())
	}

	parts := []string{lead + c.QualifiedName()}
	if c.IsGroup() {
		parts = append(parts, "<subcommand>")
	}
	for _, o := range c.Options {
		parts = append(parts, o.usage())
	}
	return strings.Join(parts, " ")
}

func (c *Command) chain() []*Command {
	var chain []*Command
	for cur := c; cur != nil; cur = cur.parent {
		chain = append(chain, cur)
	}
	slices.Reverse(chain)
	return chain
}

func (c *Command) prefixName() string {
	if !strings.ContainsAny(c.Name, " \t") {
		return strings.ToLower(c.Name)
	}

	if len(c.Aliases) > 0 {
		return strings.ToLower(c.Aliases[0])
	}

	return strings.ToLower(strings.ReplaceAll(c.Name, " ", ""))
}

func (c *Command) prefixKeys() []string {
	keys := []string{c.prefixName()}
	for _, a := range c.Aliases {
		keys = append(keys, strings.ToLower(a))
	}

	return slices.Compact(keys)
}

func (c *Command) validate(depth int) error {
	if c == nil {
		return errors.New("router: nil command")
	}

	for _, a := range c.Aliases {
		if a == "" || strings.ContainsAny(a, " \t\n") {
			return fmt.Errorf("router: %q: alias %q must be non-empty and contain no whitespace", c.Name, a)
		}
	}

	if c.Type != ChatInput {
		return c.validateContextMenu(depth)
	}

	if !validChatName(c.Name) {
		return fmt.Errorf("router: %q: chat input command names must be 1-%d lowercase characters without spaces", c.Name, maxNameLength)
	}

	if !validDescription(c.Description) {
		return fmt.Errorf("router: %q: description must be 1-%d characters", c.Name, maxDescriptionLength)
	}

	if c.IsGroup() {
		return c.validateGroup(depth)
	}

	if c.Handler == nil {
		return fmt.Errorf("router: %q: %w", c.Name, ErrNoHandler)
	}

	if len(c.Options) > maxEntries {
		return fmt.Errorf("router: %q: at most %d options are allowed", c.Name, maxEntries)
	}

	if err := validateOptions(c.Options); err != nil {
		return fmt.Errorf("router: %q: %w", c.Name, err)
	}

	return nil
}

func (c *Command) validateContextMenu(depth int) error {
	switch n := utf8.RuneCountInString(c.Name); {
	case depth > 0:
		return fmt.Errorf("router: %q: context menu commands cannot be subcommands", c.Name)
	case n < 1 || n > maxNameLength:
		return fmt.Errorf("router: %q: context menu command names must be 1-%d characters", c.Name, maxNameLength)
	case len(c.Options) > 0 || c.IsGroup():
		return fmt.Errorf("router: %q: context menu commands cannot have options or subcommands", c.Name)
	case c.Handler == nil:
		return fmt.Errorf("router: %q: %w", c.Name, ErrNoHandler)
	}

	return nil
}

// validateGroup checks a group and, recursively, its subcommands, linking
// each to its parent.
func (c *Command) validateGroup(depth int) error {
	switch {
	case depth >= 2:
		return fmt.Errorf("router: %q: subcommands can only be nested two levels deep", c.Name)
	case len(c.Options) > 0:
		return fmt.Errorf("router: %q: group commands cannot have options", c.Name)
	case c.Handler != nil:
		return fmt.Errorf("router: %q: group commands cannot have a handler", c.Name)
	case len(c.Subcommands) > maxEntries:
		return fmt.Errorf("router: %q: at most %d subcommands are allowed", c.Name, maxEntries)
	}

	seen := make(map[string]bool)
	for _, sub := range c.Subcommands {
		if sub == nil {
			return fmt.Errorf("router: %q: nil subcommand", c.Name)
		}

		if sub.Type != ChatInput {
			return fmt.Errorf("router: %q: subcommand %q must be a chat input command", c.Name, sub.Name)
		}

		sub.parent = c
		if err := sub.validate(depth + 1); err != nil {
			return err
		}

		for _, key := range append([]string{sub.Name}, sub.Aliases...) {
			key = strings.ToLower(key)
			if seen[key] {
				return fmt.Errorf("router: %q: duplicate subcommand name or alias %q", c.Name, key)
			}
			seen[key] = true
		}
	}

	return nil
}

func (c *Command) applicationCommand() *discordgo.ApplicationCommand {
	ac := &discordgo.ApplicationCommand{
		Name:                     c.Name,
		Type:                     c.Type.discord(),
		DefaultMemberPermissions: c.DefaultMemberPermissions,
	}
	if c.Type == ChatInput {
		ac.Description = c.Description
		ac.Options = c.discordOptions()
	}
	if c.NSFW {
		nsfw := true
		ac.NSFW = &nsfw
	}

	if len(c.Contexts) > 0 {
		ctxs := slices.Clone(c.Contexts)
		ac.Contexts = &ctxs
	}

	if len(c.IntegrationTypes) > 0 {
		types := slices.Clone(c.IntegrationTypes)
		ac.IntegrationTypes = &types
	}

	return ac
}

func (c *Command) discordOptions() []*discordgo.ApplicationCommandOption {
	if c.IsGroup() {
		opts := make([]*discordgo.ApplicationCommandOption, 0, len(c.Subcommands))
		for _, sub := range c.Subcommands {
			t := discordgo.ApplicationCommandOptionSubCommand
			if sub.IsGroup() {
				t = discordgo.ApplicationCommandOptionSubCommandGroup
			}
			opts = append(opts, &discordgo.ApplicationCommandOption{
				Type:        t,
				Name:        sub.Name,
				Description: sub.Description,
				Options:     sub.discordOptions(),
			})
		}

		return opts
	}

	opts := make([]*discordgo.ApplicationCommandOption, 0, len(c.Options))
	for _, o := range c.Options {
		opts = append(opts, o.toDiscord())
	}

	return opts
}
