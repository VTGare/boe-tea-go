// Package router declares a Discord command once and runs it both as a
// slash (or context menu) command and as a classic prefixed message command.
// Handlers read ctx.Options and never care how they were invoked.
package router

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/bwmarrin/discordgo"
)

// PrefixResolver returns the prefixes accepted in the given guild/channel.
type PrefixResolver func(s *discordgo.Session, guildID, channelID string) []string

// ErrorHandler receives every error returned from the command pipeline.
type ErrorHandler func(ctx *Context, err error)

// FallbackHandler receives messages (from non-bot users) that were not
// dispatched as a command.
type FallbackHandler func(s *discordgo.Session, m *discordgo.MessageCreate)

type Config struct {
	// Prefixes accepted everywhere. Ignored if PrefixResolver is set.
	Prefixes []string

	// PrefixResolver returns per-guild prefixes.
	PrefixResolver PrefixResolver

	// DisableMentionPrefix turns off "@Bot command" invocation.
	DisableMentionPrefix bool

	// DisablePrefixCommands turns off prefix invocation entirely.
	DisablePrefixCommands bool

	// DisableSlashCommands turns off interaction handling and Sync.
	DisableSlashCommands bool

	// DevGuildID registers every slash command to this guild instead of
	// globally (instant propagation, useful for development).
	DevGuildID string

	// ApplicationID used for Sync. Fetched from the API if empty.
	ApplicationID string

	OwnerIDs []string

	// ErrorHandler replaces the default error handler.
	ErrorHandler ErrorHandler

	// Fallback is called for messages that were not commands.
	Fallback FallbackHandler

	// AllowedMentions applied to prefix replies that don't set their own.
	AllowedMentions *discordgo.MessageAllowedMentions

	// DisableReplyReference stops prefix replies from quoting the invoking
	// message.
	DisableReplyReference bool

	// BaseContext produces the root context.Context for each invocation.
	BaseContext func() context.Context
}

type slashKey struct {
	t    CommandType
	name string
}

type Router struct {
	cfg Config

	mu          sync.RWMutex
	commands    []*Command
	prefixIndex map[string]*Command
	slashIndex  map[slashKey]*Command
	middleware  []Middleware
	checks      []Check
	owners      map[string]struct{}
	// commandIDs are the synced application command IDs, for mentions.
	commandIDs map[slashKey]string
}

func New(cfg Config) *Router {
	r := &Router{
		cfg:         cfg,
		prefixIndex: make(map[string]*Command),
		slashIndex:  make(map[slashKey]*Command),
		owners:      make(map[string]struct{}),
		commandIDs:  make(map[slashKey]string),
	}
	for _, id := range cfg.OwnerIDs {
		r.owners[id] = struct{}{}
	}
	return r
}

func (r *Router) Config() Config { return r.cfg }

func (r *Router) IsOwner(userID string) bool {
	_, ok := r.owners[userID]
	return ok
}

// Use appends router-wide middleware. Middleware runs in registration order.
func (r *Router) Use(mw ...Middleware) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.middleware = append(r.middleware, mw...)
}

// AddCheck appends router-wide checks that run before every command.
func (r *Router) AddCheck(checks ...Check) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.checks = append(r.checks, checks...)
}

// Register validates and adds top-level commands. Either all commands are
// registered or none.
func (r *Router) Register(cmds ...*Command) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	newPrefix := make(map[string]*Command)
	newSlash := make(map[slashKey]*Command)

	for _, c := range cmds {
		if err := c.validate(0); err != nil {
			return err
		}
		c.parent = nil

		if !c.DisablePrefix {
			for _, key := range c.prefixKeys() {
				if _, ok := r.prefixIndex[key]; ok {
					return fmt.Errorf("router: prefix name %q is already registered", key)
				}
				if _, ok := newPrefix[key]; ok {
					return fmt.Errorf("router: prefix name %q is registered twice", key)
				}
				newPrefix[key] = c
			}
		}
		if !c.DisableSlash {
			key := slashKey{c.Type, c.Name}
			if _, ok := r.slashIndex[key]; ok {
				return fmt.Errorf("router: %s command %q is already registered", c.Type, c.Name)
			}
			if _, ok := newSlash[key]; ok {
				return fmt.Errorf("router: %s command %q is registered twice", c.Type, c.Name)
			}
			newSlash[key] = c
		}
	}

	maps.Copy(r.prefixIndex, newPrefix)
	maps.Copy(r.slashIndex, newSlash)

	r.commands = append(r.commands, cmds...)
	return nil
}

// MustRegister is Register but panics on error.
func (r *Router) MustRegister(cmds ...*Command) {
	if err := r.Register(cmds...); err != nil {
		panic(err)
	}
}

func (r *Router) Commands() []*Command {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]*Command, len(r.commands))
	copy(out, r.commands)
	return out
}

// Lookup resolves a command by prefix path, e.g. Lookup("set", "prefix").
// Names and aliases both match; nil if not found.
func (r *Router) Lookup(path ...string) *Command {
	if len(path) == 0 {
		return nil
	}
	r.mu.RLock()
	cmd := r.prefixIndex[strings.ToLower(path[0])]
	r.mu.RUnlock()

	if cmd == nil {
		// Fall back to slash names so Lookup works for DisablePrefix commands.
		for _, c := range r.Commands() {
			if strings.EqualFold(c.Name, path[0]) {
				cmd = c
				break
			}
		}
	}
	for _, p := range path[1:] {
		if cmd == nil {
			return nil
		}
		cmd = cmd.Subcommand(p)
	}
	return cmd
}

func (r *Router) Prefixes(s *discordgo.Session, guildID, channelID string) []string {
	if r.cfg.PrefixResolver != nil {
		return r.cfg.PrefixResolver(s, guildID, channelID)
	}

	return r.cfg.Prefixes
}

// mentionPrefix is how mention invocation ("@Bot ") reads, or "" when it
// is off or the bot user is unknown.
func (r *Router) mentionPrefix(s *discordgo.Session) string {
	if r.cfg.DisableMentionPrefix || s == nil || s.State == nil || s.State.User == nil {
		return ""
	}

	return "@" + s.State.User.Username + " "
}

// Bind hooks the router into the session; the result unbinds it.
func (r *Router) Bind(s *discordgo.Session) func() {
	rmMsg := s.AddHandler(r.HandleMessage)
	rmInt := s.AddHandler(r.HandleInteraction)
	return func() {
		rmMsg()
		rmInt()
	}
}

func (r *Router) HandleMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m == nil || m.Message == nil || m.Author == nil || m.Author.Bot {
		return
	}
	if !r.dispatchMessage(s, m) && r.cfg.Fallback != nil {
		r.cfg.Fallback(s, m)
	}
}

// dispatchMessage returns true if the message was handled as a command.
func (r *Router) dispatchMessage(s *discordgo.Session, m *discordgo.MessageCreate) bool {
	if r.cfg.DisablePrefixCommands {
		return false
	}

	prefix, rest, ok := r.matchPrefix(s, m)
	if !ok {
		return false
	}
	rest = strings.TrimSpace(rest)
	if rest == "" {
		return false
	}

	toks := tokenize(rest)
	r.mu.RLock()
	cmd := r.prefixIndex[strings.ToLower(toks[0].text)]
	r.mu.RUnlock()

	if cmd == nil || cmd.DisablePrefix {
		return false
	}

	var parseErr error
	i := 1
	for cmd.IsGroup() {
		if i >= len(toks) {
			parseErr = &SubcommandError{Command: cmd}
			break
		}
		sub := cmd.Subcommand(toks[i].text)
		if sub == nil || sub.DisablePrefix {
			parseErr = &SubcommandError{Command: cmd, Given: toks[i].text}
			break
		}
		cmd = sub
		i++
	}

	args := ""
	if i < len(toks) {
		args = rest[toks[i].start:]
	}

	ctx := newContext(r, s, cmd)
	ctx.Source = SourceMessage
	ctx.Prefix = prefix
	ctx.Message = m.Message

	if parseErr == nil {
		switch cmd.Type {
		case ChatInput:
			ctx.Options, parseErr = parsePrefixOptions(ctx, cmd, args)
		case MessageContext:
			parseErr = resolveMessageTarget(ctx, m, args)
		case UserContext:
			parseErr = resolveUserTarget(ctx, m, args)
		}
	}

	r.dispatch(ctx, parseErr)
	return true
}

func (r *Router) matchPrefix(s *discordgo.Session, m *discordgo.MessageCreate) (prefix, rest string, ok bool) {
	content := m.Content

	if !r.cfg.DisableMentionPrefix && s.State != nil && s.State.User != nil {
		id := s.State.User.ID
		for _, p := range []string{"<@" + id + ">", "<@!" + id + ">"} {
			if strings.HasPrefix(content, p) {
				return p, content[len(p):], true
			}
		}
	}

	// The longest match wins, so "bt!" beats "bt" for "bt!help".
	for _, p := range r.Prefixes(s, m.GuildID, m.ChannelID) {
		if p != "" && len(p) > len(prefix) && len(content) >= len(p) && strings.EqualFold(content[:len(p)], p) {
			prefix = p
		}
	}

	if prefix == "" {
		return "", "", false
	}

	return prefix, content[len(prefix):], true
}

func resolveMessageTarget(ctx *Context, m *discordgo.MessageCreate, args string) error {
	if m.ReferencedMessage != nil {
		ctx.TargetMessage = m.ReferencedMessage
		return nil
	}
	if toks := tokenize(args); len(toks) > 0 {
		channelID, messageID, ok := parseMessageRef(toks[0].text, m.ChannelID)
		if !ok {
			return NewUserError("Reply to a message or provide a message link to use this command.")
		}
		msg, err := ctx.Session.ChannelMessage(channelID, messageID)
		if err != nil {
			return WrapUserError("I couldn't find that message.", err)
		}
		ctx.TargetMessage = msg
		return nil
	}
	return NewUserError("Reply to a message or provide a message link to use this command.")
}

func resolveUserTarget(ctx *Context, m *discordgo.MessageCreate, args string) error {
	toks := tokenize(args)
	if len(toks) == 0 {
		ctx.TargetUser = m.Author
		ctx.TargetMember = ctx.Member()
		return nil
	}

	id := parseMention(toks[0].text, userMentionRe)
	if id == "" {
		return NewUserError("Expected a user mention or ID.")
	}
	v := &Value{Type: OptionUser, id: id, kind: kindUser, s: ctx.Session, guildID: m.GuildID}
	if m.GuildID != "" {
		if member, err := v.Member(); err == nil {
			ctx.TargetMember = member
			ctx.TargetUser = member.User
			return nil
		}
	}
	user, err := v.User()
	if err != nil {
		return WrapUserError("I couldn't find that user.", err)
	}
	ctx.TargetUser = user
	return nil
}

// HandleInteraction runs application commands and the components and
// modals built with ComponentID. It ignores everything else, so it can
// share a session with your own component and modal handlers.
func (r *Router) HandleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i == nil || i.Interaction == nil {
		return
	}
	switch {
	case i.Type == discordgo.InteractionMessageComponent || i.Type == discordgo.InteractionModalSubmit:
		r.handleComponent(s, i)
	case i.Type == discordgo.InteractionApplicationCommand && !r.cfg.DisableSlashCommands:
		r.handleCommand(s, i)
	}
}

func (r *Router) handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	data := i.ApplicationCommandData()

	r.mu.RLock()
	cmd := r.slashIndex[slashKey{commandTypeOf(data.CommandType), data.Name}]
	r.mu.RUnlock()

	if cmd == nil {
		return
	}

	var parseErr error
	opts := data.Options
	for cmd.IsGroup() {
		if len(opts) == 0 {
			parseErr = &SubcommandError{Command: cmd}
			break
		}
		sub := cmd.Subcommand(opts[0].Name)
		if sub == nil {
			parseErr = &SubcommandError{Command: cmd, Given: opts[0].Name}
			break
		}
		cmd = sub
		opts = opts[0].Options
	}

	ctx := newContext(r, s, cmd)
	ctx.Source = SourceInteraction
	ctx.Interaction = i

	if parseErr == nil {
		switch cmd.Type {
		case ChatInput:
			ctx.Options = optionsFromInteraction(s, i.GuildID, opts, data.Resolved)
		case MessageContext:
			if data.Resolved != nil {
				ctx.TargetMessage = data.Resolved.Messages[data.TargetID]
			}
			if ctx.TargetMessage == nil {
				parseErr = ErrMissingTarget
			}
		case UserContext:
			if data.Resolved != nil {
				ctx.TargetUser = data.Resolved.Users[data.TargetID]
				ctx.TargetMember = data.Resolved.Members[data.TargetID]
				if ctx.TargetMember != nil && ctx.TargetMember.User == nil {
					ctx.TargetMember.User = ctx.TargetUser
				}
			}
			if ctx.TargetUser == nil {
				parseErr = ErrMissingTarget
			}
		}
	}

	r.dispatch(ctx, parseErr)
}

// dispatch runs middleware, then checks and cooldowns, then the handler.
// Parse errors go through the middleware too, so logging sees them.
func (r *Router) dispatch(ctx *Context, parseErr error) {
	cmd := ctx.Command
	chain := cmd.chain()

	r.mu.RLock()
	mws := slices.Clone(r.middleware)
	checks := slices.Clone(r.checks)
	r.mu.RUnlock()

	for _, c := range chain {
		mws = append(mws, c.Middleware...)
		checks = append(checks, c.Checks...)
	}

	var h Handler = func(ctx *Context) error {
		if parseErr != nil {
			return parseErr
		}
		for _, chk := range checks {
			if err := chk(ctx); err != nil {
				return err
			}
		}
		for _, c := range chain {
			if c.Cooldown == nil {
				continue
			}
			if remaining, ok := c.Cooldown.Take(c.Cooldown.Key(ctx)); !ok {
				return &CooldownError{Remaining: remaining, Scope: c.Cooldown.Scope}
			}
		}
		if cmd.Defer {
			if err := ctx.Defer(); err != nil {
				return err
			}
		}
		if cmd.Handler == nil {
			return ErrNoHandler
		}
		return cmd.Handler(ctx)
	}

	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}

	if err := h(ctx); err != nil {
		r.handleError(ctx, err)
	}
}

func (r *Router) handleError(ctx *Context, err error) {
	if r.cfg.ErrorHandler != nil {
		r.cfg.ErrorHandler(ctx, err)
		return
	}

	DefaultErrorHandler(ctx, err)
}

// DefaultErrorHandler replies with the user-facing message, if any, and a
// generic one otherwise. Silent check failures get no reply.
func DefaultErrorHandler(ctx *Context, err error) {
	if ce, ok := errors.AsType[*CheckError](err); ok && ce.Silent {
		return
	}

	if msg, ok := UserMessageOf(err); ok {
		_ = ctx.Reply(Text(msg).Private())
		return
	}
	_ = ctx.Reply(Text("Something went wrong while running this command.").Private())
}

// ApplicationCommands builds the Discord definitions, split into global and
// per-guild commands. DevGuildID is not applied here; see Sync.
func (r *Router) ApplicationCommands() (global []*discordgo.ApplicationCommand, byGuild map[string][]*discordgo.ApplicationCommand) {
	byGuild = make(map[string][]*discordgo.ApplicationCommand)
	for _, c := range r.Commands() {
		if c.DisableSlash {
			continue
		}
		ac := c.applicationCommand()
		if len(c.GuildIDs) == 0 {
			global = append(global, ac)
			continue
		}
		for _, gid := range c.GuildIDs {
			byGuild[gid] = append(byGuild[gid], ac)
		}
	}
	return global, byGuild
}

// Sync bulk-overwrites the application commands. With DevGuildID set,
// everything lands in that guild instead (globals are left alone).
// Call it after the session is open.
func (r *Router) Sync(s *discordgo.Session) error {
	if r.cfg.DisableSlashCommands {
		return nil
	}
	appID, err := r.applicationID(s)
	if err != nil {
		return fmt.Errorf("router: resolve application id: %w", err)
	}

	global, byGuild := r.ApplicationCommands()
	if r.cfg.DevGuildID != "" {
		byGuild[r.cfg.DevGuildID] = dedupeCommands(append(byGuild[r.cfg.DevGuildID], global...))
		global = nil
	} else {
		synced, err := s.ApplicationCommandBulkOverwrite(appID, "", global)
		if err != nil {
			return fmt.Errorf("router: sync global commands: %w", err)
		}
		r.storeCommandIDs(synced)
	}

	for gid, cmds := range byGuild {
		synced, err := s.ApplicationCommandBulkOverwrite(appID, gid, cmds)
		if err != nil {
			return fmt.Errorf("router: sync commands for guild %s: %w", gid, err)
		}
		// Per-guild IDs differ between guilds, so only the dev guild's
		// (which holds everything) are usable for mentions.
		if gid == r.cfg.DevGuildID {
			r.storeCommandIDs(synced)
		}
	}
	return nil
}

func (r *Router) storeCommandIDs(cmds []*discordgo.ApplicationCommand) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, c := range cmds {
		r.commandIDs[slashKey{commandTypeOf(c.Type), c.Name}] = c.ID
	}
}

// Mention renders a chat command as a clickable </name:id> mention, or ""
// before Sync or for commands Discord cannot mention.
func (r *Router) Mention(c *Command) string {
	if c.Type != ChatInput || !slashEnabled(r, c) {
		return ""
	}
	r.mu.RLock()
	id := r.commandIDs[slashKey{ChatInput, c.Root().Name}]
	r.mu.RUnlock()
	if id == "" {
		return ""
	}
	return "</" + c.QualifiedName() + ":" + id + ">"
}

// ClearCommands wipes global (guildID == "") or guild commands.
func (r *Router) ClearCommands(s *discordgo.Session, guildID string) error {
	appID, err := r.applicationID(s)
	if err != nil {
		return err
	}

	_, err = s.ApplicationCommandBulkOverwrite(appID, guildID, []*discordgo.ApplicationCommand{})
	return err
}

func (r *Router) applicationID(s *discordgo.Session) (string, error) {
	if r.cfg.ApplicationID != "" {
		return r.cfg.ApplicationID, nil
	}
	app, err := s.Application("@me")
	if err != nil {
		return "", err
	}
	return app.ID, nil
}

func dedupeCommands(cmds []*discordgo.ApplicationCommand) []*discordgo.ApplicationCommand {
	type key struct {
		t    discordgo.ApplicationCommandType
		name string
	}
	seen := make(map[key]bool)
	out := cmds[:0]
	for _, c := range cmds {
		k := key{c.Type, c.Name}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, c)
	}
	return out
}
