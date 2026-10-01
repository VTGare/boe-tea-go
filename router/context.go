package router

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

type Source int

const (
	SourceMessage Source = iota
	SourceInteraction
)

func (s Source) String() string {
	switch s {
	case SourceMessage:
		return "message"
	case SourceInteraction:
		return "interaction"
	}
	return "unknown"
}

// Context is a single command invocation. Replies work the same whether
// it came from a slash command or a message. Exactly one of Message and
// Interaction is set.
type Context struct {
	Session       *discordgo.Session
	Router        *Router
	Command       *Command
	Source        Source
	Prefix        string
	Message       *discordgo.Message
	Interaction   *discordgo.InteractionCreate
	Options       *Options
	TargetMessage *discordgo.Message
	TargetUser    *discordgo.User
	// Guilds only; nil in DMs.
	TargetMember *discordgo.Member
	StartedAt    time.Time

	ctx    context.Context
	mu     sync.Mutex
	values map[string]any

	responded   bool
	deferred    bool
	edited      bool
	ephemeral   bool
	lastMessage *discordgo.Message
}

func newContext(r *Router, s *discordgo.Session, cmd *Command) *Context {
	base := context.Background()
	if r.cfg.BaseContext != nil {
		base = r.cfg.BaseContext()
	}
	return &Context{
		Session:   s,
		Router:    r,
		Command:   cmd,
		Options:   newOptions(),
		StartedAt: time.Now(),
		ctx:       base,
		ephemeral: cmd != nil && cmd.Ephemeral,
	}
}

// Context carries cancellation and deadlines; see middleware.Timeout.
func (c *Context) Context() context.Context {
	if c.ctx == nil {
		return context.Background()
	}
	return c.ctx
}

func (c *Context) SetContext(ctx context.Context) { c.ctx = ctx }

// Set stashes a value for middleware and handlers to share.
func (c *Context) Set(key string, value any) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.values == nil {
		c.values = make(map[string]any)
	}

	c.values[key] = value
}

func (c *Context) Get(key string) (any, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.values[key]
	return v, ok
}

func (c *Context) IsInteraction() bool { return c.Interaction != nil }

func (c *Context) IsMessage() bool { return c.Message != nil }

func (c *Context) GuildID() string {
	if c.Interaction != nil {
		return c.Interaction.GuildID
	}

	if c.Message != nil {
		return c.Message.GuildID
	}

	return ""
}

func (c *Context) ChannelID() string {
	if c.Interaction != nil {
		return c.Interaction.ChannelID
	}

	if c.Message != nil {
		return c.Message.ChannelID
	}

	return ""
}

func (c *Context) Author() *discordgo.User {
	if c.Interaction != nil {
		if c.Interaction.Member != nil && c.Interaction.Member.User != nil {
			return c.Interaction.Member.User
		}

		return c.Interaction.User
	}

	if c.Message != nil {
		return c.Message.Author
	}

	return nil
}

func (c *Context) AuthorID() string {
	if u := c.Author(); u != nil {
		return u.ID
	}

	return ""
}

func (c *Context) Member() *discordgo.Member {
	if c.Interaction != nil {
		return c.Interaction.Member
	}

	if c.Message != nil && c.Message.Member != nil {
		m := c.Message.Member
		if m.User == nil {
			m.User = c.Message.Author
		}

		if m.GuildID == "" {
			m.GuildID = c.Message.GuildID
		}

		return m
	}

	return nil
}

func (c *Context) Locale() discordgo.Locale {
	if c.Interaction != nil {
		return c.Interaction.Locale
	}

	return ""
}

// Channel fetches the invoking channel (state cache first, then REST).
func (c *Context) Channel() (*discordgo.Channel, error) {
	return c.channel(c.ChannelID())
}

func (c *Context) channel(id string) (*discordgo.Channel, error) {
	if c.Session.State != nil {
		if ch, err := c.Session.State.Channel(id); err == nil {
			return ch, nil
		}
	}

	return c.Session.Channel(id)
}

// Guild fetches the invoking guild (state cache first, then REST).
func (c *Context) Guild() (*discordgo.Guild, error) {
	id := c.GuildID()
	if id == "" {
		return nil, fmt.Errorf("router: not in a guild")
	}

	if c.Session.State != nil {
		if g, err := c.Session.State.Guild(id); err == nil {
			return g, nil
		}
	}

	return c.Session.Guild(id)
}

// Permissions are the invoker's channel permissions (all of them outside guilds).
func (c *Context) Permissions() (int64, error) {
	if c.GuildID() == "" {
		return discordgo.PermissionAll, nil
	}

	if c.Interaction != nil && c.Interaction.Member != nil {
		return c.Interaction.Member.Permissions, nil
	}

	return c.userPermissions(c.AuthorID())
}

func (c *Context) BotPermissions() (int64, error) {
	if c.GuildID() == "" {
		return discordgo.PermissionAll, nil
	}

	if c.Interaction != nil {
		return c.Interaction.AppPermissions, nil
	}

	if c.Session.State == nil || c.Session.State.User == nil {
		return 0, fmt.Errorf("router: bot user unknown")
	}

	return c.userPermissions(c.Session.State.User.ID)
}

func (c *Context) userPermissions(userID string) (int64, error) {
	if c.Session.State != nil {
		if p, err := c.Session.State.UserChannelPermissions(userID, c.ChannelID()); err == nil {
			return p, nil
		}
	}

	return c.Session.UserChannelPermissions(userID, c.ChannelID())
}

// DisplayPrefix is the used prefix, falling back to the guild's first one.
func (c *Context) DisplayPrefix() string {
	if c.Prefix != "" {
		return c.Prefix
	}

	if p := c.Router.Prefixes(c.Session, c.GuildID(), c.ChannelID()); len(p) > 0 {
		return p[0]
	}

	return c.Router.mentionPrefix(c.Session)
}

func (c *Context) messageAttachments() []*discordgo.MessageAttachment {
	if c.Message != nil {
		return c.Message.Attachments
	}
	return nil
}

type Response struct {
	Content         string
	Embeds          []*discordgo.MessageEmbed
	Components      []discordgo.MessageComponent
	Files           []*discordgo.File
	AllowedMentions *discordgo.MessageAllowedMentions
	// Invoker-only for interactions; ignored over prefix.
	Ephemeral bool
	TTS       bool
	Flags     discordgo.MessageFlags
}

func Text(content string) *Response { return &Response{Content: content} }

func Textf(format string, args ...any) *Response {
	return &Response{Content: fmt.Sprintf(format, args...)}
}

func Embed(embed *discordgo.MessageEmbed) *Response {
	return &Response{Embeds: []*discordgo.MessageEmbed{embed}}
}

func (r *Response) Private() *Response { r.Ephemeral = true; return r }

func (r *Response) flags(ephemeral bool) discordgo.MessageFlags {
	f := r.Flags
	if ephemeral || r.Ephemeral {
		f |= discordgo.MessageFlagsEphemeral
	}
	return f
}

func (r *Response) interactionData(ephemeral bool) *discordgo.InteractionResponseData {
	return &discordgo.InteractionResponseData{
		TTS:             r.TTS,
		Content:         r.Content,
		Components:      r.Components,
		Embeds:          r.Embeds,
		AllowedMentions: r.AllowedMentions,
		Files:           r.Files,
		Flags:           r.flags(ephemeral),
	}
}

func (r *Response) webhookParams(ephemeral bool) *discordgo.WebhookParams {
	return &discordgo.WebhookParams{
		Content:         r.Content,
		TTS:             r.TTS,
		Files:           r.Files,
		Components:      r.Components,
		Embeds:          r.Embeds,
		AllowedMentions: r.AllowedMentions,
		Flags:           r.flags(ephemeral),
	}
}

// editFields is the edit payload. Unset embeds and components become empty
// so the edit clears them instead of keeping the old ones.
func (r *Response) editFields() (*string, *[]*discordgo.MessageEmbed, *[]discordgo.MessageComponent) {
	content := r.Content
	embeds := r.Embeds
	if embeds == nil {
		embeds = []*discordgo.MessageEmbed{}
	}

	components := r.Components
	if components == nil {
		components = []discordgo.MessageComponent{}
	}

	return &content, &embeds, &components
}

func (r *Response) webhookEdit() *discordgo.WebhookEdit {
	content, embeds, components := r.editFields()

	return &discordgo.WebhookEdit{
		Content:         content,
		Embeds:          embeds,
		Components:      components,
		Files:           r.Files,
		AllowedMentions: r.AllowedMentions,
	}
}

func (r *Response) messageSend(ref *discordgo.MessageReference, defaultMentions *discordgo.MessageAllowedMentions) *discordgo.MessageSend {
	am := r.AllowedMentions
	if am == nil {
		am = defaultMentions
	}

	return &discordgo.MessageSend{
		Content:         r.Content,
		Embeds:          r.Embeds,
		TTS:             r.TTS,
		Components:      r.Components,
		Files:           r.Files,
		AllowedMentions: am,
		Reference:       ref,
		Flags:           r.Flags &^ discordgo.MessageFlagsEphemeral,
	}
}

func (r *Response) messageEdit(channelID, messageID string) *discordgo.MessageEdit {
	content, embeds, components := r.editFields()

	return &discordgo.MessageEdit{
		ID:              messageID,
		Channel:         channelID,
		Content:         content,
		Embeds:          embeds,
		Components:      components,
		Files:           r.Files,
		AllowedMentions: r.AllowedMentions,
	}
}

// Reply answers once over interactions (editing the deferred placeholder
// if there is one, else following up), or replies to the message.
func (c *Context) Reply(r *Response) error {
	if r == nil {
		r = &Response{}
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Interaction != nil {
		switch {
		case !c.responded:
			err := c.Session.InteractionRespond(c.Interaction.Interaction, &discordgo.InteractionResponse{
				Type: discordgo.InteractionResponseChannelMessageWithSource,
				Data: r.interactionData(c.ephemeral),
			})
			if err == nil {
				c.responded = true
			}

			return err
		case c.deferred && !c.edited:
			msg, err := c.Session.InteractionResponseEdit(c.Interaction.Interaction, r.webhookEdit())
			if err == nil {
				c.edited = true
				c.lastMessage = msg
			}

			return err
		default:
			msg, err := c.Session.FollowupMessageCreate(c.Interaction.Interaction, true, r.webhookParams(c.ephemeral))
			if err == nil {
				c.lastMessage = msg
			}

			return err
		}
	}

	var ref *discordgo.MessageReference
	if c.Message != nil && !c.Router.cfg.DisableReplyReference {
		ref = c.Message.SoftReference()
	}

	msg, err := c.Session.ChannelMessageSendComplex(c.ChannelID(), r.messageSend(ref, c.Router.cfg.AllowedMentions))
	if err == nil {
		c.responded = true
		c.lastMessage = msg
	}

	return err
}

func (c *Context) ReplyText(content string) error { return c.Reply(Text(content)) }

func (c *Context) Replyf(format string, args ...any) error {
	return c.Reply(Textf(format, args...))
}

func (c *Context) ReplyEmbed(embed *discordgo.MessageEmbed) error { return c.Reply(Embed(embed)) }

func (c *Context) ReplyEphemeral(content string) error { return c.Reply(Text(content).Private()) }

// Defer gives the handler up to 15 minutes by showing "thinking…" (or
// typing, over prefix). Calling it twice does nothing.
func (c *Context) Defer() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Interaction != nil {
		if c.responded {
			return nil
		}

		var data *discordgo.InteractionResponseData
		if c.ephemeral {
			data = &discordgo.InteractionResponseData{Flags: discordgo.MessageFlagsEphemeral}
		}

		err := c.Session.InteractionRespond(c.Interaction.Interaction, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
			Data: data,
		})
		if err == nil {
			c.responded, c.deferred = true, true
		}

		return err
	}

	if c.Message != nil {
		return c.Session.ChannelTyping(c.ChannelID())
	}

	return nil
}

// Edit rewrites the original response, or the last reply over prefix.
// Replies instead if nothing was sent yet.
func (c *Context) Edit(r *Response) error {
	if r == nil {
		r = &Response{}
	}

	if !c.Responded() {
		return c.Reply(r)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.Interaction != nil {
		msg, err := c.Session.InteractionResponseEdit(c.Interaction.Interaction, r.webhookEdit())
		if err == nil {
			c.edited = true
			c.lastMessage = msg
		}

		return err
	}

	if c.lastMessage == nil {
		return ErrNoResponse
	}

	msg, err := c.Session.ChannelMessageEditComplex(r.messageEdit(c.lastMessage.ChannelID, c.lastMessage.ID))
	if err == nil {
		c.lastMessage = msg
	}

	return err
}

// Followup sends an extra message, responding first if it has to.
func (c *Context) Followup(r *Response) (*discordgo.Message, error) {
	if r == nil {
		r = &Response{}
	}

	if c.Interaction != nil {
		if !c.Responded() {
			if err := c.Reply(r); err != nil {
				return nil, err
			}

			return c.OriginalResponse()
		}

		return c.Session.FollowupMessageCreate(c.Interaction.Interaction, true, r.webhookParams(c.ephemeral))
	}

	return c.Session.ChannelMessageSendComplex(c.ChannelID(), r.messageSend(nil, c.Router.cfg.AllowedMentions))
}

func (c *Context) OriginalResponse() (*discordgo.Message, error) {
	if c.Interaction != nil {
		return c.Session.InteractionResponse(c.Interaction.Interaction)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.lastMessage == nil {
		return nil, ErrNoResponse
	}

	return c.lastMessage, nil
}

// Delete removes the original response (interactions) or the last reply.
func (c *Context) Delete() error {
	if c.Interaction != nil {
		return c.Session.InteractionResponseDelete(c.Interaction.Interaction)
	}

	c.mu.Lock()
	msg := c.lastMessage
	c.mu.Unlock()

	if msg == nil {
		return ErrNoResponse
	}

	return c.Session.ChannelMessageDelete(msg.ChannelID, msg.ID)
}

// Responded reports whether an initial response (or Defer) has been sent.
func (c *Context) Responded() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.responded
}
