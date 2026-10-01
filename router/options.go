package router

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/bwmarrin/discordgo"
)

type OptionType = discordgo.ApplicationCommandOptionType

// Subcommand types are deliberately absent: use Command.Subcommands.
const (
	OptionString      OptionType = discordgo.ApplicationCommandOptionString
	OptionInteger     OptionType = discordgo.ApplicationCommandOptionInteger
	OptionBoolean     OptionType = discordgo.ApplicationCommandOptionBoolean
	OptionUser        OptionType = discordgo.ApplicationCommandOptionUser
	OptionChannel     OptionType = discordgo.ApplicationCommandOptionChannel
	OptionRole        OptionType = discordgo.ApplicationCommandOptionRole
	OptionMentionable OptionType = discordgo.ApplicationCommandOptionMentionable
	OptionNumber      OptionType = discordgo.ApplicationCommandOptionNumber
	OptionAttachment  OptionType = discordgo.ApplicationCommandOptionAttachment
)

type Choice struct {
	Name  string
	Value any
}

// Option is one command argument; the same declaration feeds the slash
// definition and the prefix parser.
type Option struct {
	Type        OptionType
	Name        string
	Description string
	Required    bool
	Choices     []Choice
	// ChannelTypes restricts channel options to the given types.
	ChannelTypes []discordgo.ChannelType
	// MinValue / MaxValue apply to integer and number options.
	MinValue *float64
	MaxValue *float64
	// MinLength / MaxLength apply to string options.
	MinLength *int
	MaxLength *int
	// Consume the rest of the message over prefix (string only). Only
	// attachments and slash-only options may follow it. Ignored by slash
	// commands.
	Rest bool
	// Skip the option over prefix, where it keeps its zero value. Lets a
	// slash command take extra options without breaking the positional
	// prefix syntax. Cannot be required.
	NoPrefix bool
}

// Declare options fluently, e.g.
//
//	router.String("query", "What to search for").Require().Greedy()

func String(name, description string) *Option {
	return &Option{Type: OptionString, Name: name, Description: description}
}

func Integer(name, description string) *Option {
	return &Option{Type: OptionInteger, Name: name, Description: description}
}

func Number(name, description string) *Option {
	return &Option{Type: OptionNumber, Name: name, Description: description}
}

func Boolean(name, description string) *Option {
	return &Option{Type: OptionBoolean, Name: name, Description: description}
}

func User(name, description string) *Option {
	return &Option{Type: OptionUser, Name: name, Description: description}
}

func Channel(name, description string) *Option {
	return &Option{Type: OptionChannel, Name: name, Description: description}
}

func Role(name, description string) *Option {
	return &Option{Type: OptionRole, Name: name, Description: description}
}

func Mentionable(name, description string) *Option {
	return &Option{Type: OptionMentionable, Name: name, Description: description}
}

func Attachment(name, description string) *Option {
	return &Option{Type: OptionAttachment, Name: name, Description: description}
}

func (o *Option) Require() *Option { o.Required = true; return o }

func (o *Option) Greedy() *Option { o.Rest = true; return o }

func (o *Option) SlashOnly() *Option { o.NoPrefix = true; return o }

func (o *Option) WithChoices(choices ...Choice) *Option { o.Choices = choices; return o }

func (o *Option) WithRange(minValue, maxValue float64) *Option {
	o.MinValue, o.MaxValue = &minValue, &maxValue
	return o
}

func (o *Option) WithLength(minLength, maxLength int) *Option {
	o.MinLength, o.MaxLength = &minLength, &maxLength
	return o
}

func (o *Option) WithChannelTypes(types ...discordgo.ChannelType) *Option {
	o.ChannelTypes = types
	return o
}

func (o *Option) usage() string {
	name := o.Name
	if o.Rest {
		name += "..."
	}
	if o.Required {
		return "<" + name + ">"
	}
	return "[" + name + "]"
}

func (o *Option) toDiscord() *discordgo.ApplicationCommandOption {
	d := &discordgo.ApplicationCommandOption{
		Type:         o.Type,
		Name:         o.Name,
		Description:  o.Description,
		Required:     o.Required,
		ChannelTypes: o.ChannelTypes,
		MinValue:     o.MinValue,
		MinLength:    o.MinLength,
	}
	if o.MaxValue != nil {
		d.MaxValue = *o.MaxValue
	}
	if o.MaxLength != nil {
		d.MaxLength = *o.MaxLength
	}
	for _, c := range o.Choices {
		d.Choices = append(d.Choices, &discordgo.ApplicationCommandOptionChoice{Name: c.Name, Value: c.Value})
	}
	return d
}

func validateOptions(opts []*Option) error {
	seenOptional := false
	names := make(map[string]bool)

	for i, o := range opts {
		if o == nil {
			return errors.New("nil option")
		}
		if !validChatName(o.Name) {
			return fmt.Errorf("option %q: names must be 1-%d lowercase characters without spaces", o.Name, maxNameLength)
		}

		if !validDescription(o.Description) {
			return fmt.Errorf("option %q: description must be 1-%d characters", o.Name, maxDescriptionLength)
		}
		if names[o.Name] {
			return fmt.Errorf("option %q: duplicate option name", o.Name)
		}
		names[o.Name] = true

		switch o.Type {
		case OptionString, OptionInteger, OptionBoolean, OptionUser, OptionChannel,
			OptionRole, OptionMentionable, OptionNumber, OptionAttachment:
		default:
			return fmt.Errorf("option %q: unsupported option type %d (use Subcommands for subcommands)", o.Name, o.Type)
		}

		if o.Required && seenOptional {
			return fmt.Errorf("option %q: required options must be declared before optional ones", o.Name)
		}

		if !o.Required {
			seenOptional = true
		}

		if o.NoPrefix && o.Required {
			return fmt.Errorf("option %q: slash-only options cannot be required", o.Name)
		}

		if o.Rest {
			if o.Type != OptionString {
				return fmt.Errorf("option %q: only string options can be greedy", o.Name)
			}

			for _, after := range opts[i+1:] {
				if after != nil && after.Type != OptionAttachment && !after.NoPrefix {
					return fmt.Errorf("option %q: only attachments and slash-only options may follow a greedy option", o.Name)
				}
			}
		}
		if len(o.Choices) > maxEntries {
			return fmt.Errorf("option %q: at most %d choices are allowed", o.Name, maxEntries)
		}
	}
	return nil
}

type Options struct {
	values map[string]*Value
}

func newOptions() *Options { return &Options{values: make(map[string]*Value)} }

func (o *Options) set(name string, v *Value) { o.values[name] = v }

func (o *Options) Has(name string) bool {
	_, ok := o.values[name]
	return ok
}

func (o *Options) Len() int { return len(o.values) }

func (o *Options) Get(name string) *Value { return o.values[name] }

func (o *Options) String(name string) string { return o.Get(name).String() }

func (o *Options) StringOr(name, def string) string {
	if v := o.Get(name); v != nil {
		return v.String()
	}
	return def
}

func (o *Options) Int(name string) int64 { return o.Get(name).Int() }

func (o *Options) IntOr(name string, def int64) int64 {
	if v := o.Get(name); v != nil {
		return v.Int()
	}
	return def
}

func (o *Options) Float(name string) float64 { return o.Get(name).Float() }

func (o *Options) Bool(name string) bool { return o.Get(name).Bool() }

func (o *Options) BoolOr(name string, def bool) bool {
	if v := o.Get(name); v != nil {
		return v.Bool()
	}
	return def
}

func (o *Options) ID(name string) string { return o.Get(name).ID() }

// Entity getters return nil when the option is absent or unresolvable;
// call the Value's method directly to see why.

func (o *Options) User(name string) *discordgo.User {
	u, _ := o.Get(name).User()
	return u
}

func (o *Options) Member(name string) *discordgo.Member {
	m, _ := o.Get(name).Member()
	return m
}

func (o *Options) Channel(name string) *discordgo.Channel {
	c, _ := o.Get(name).Channel()
	return c
}

func (o *Options) Role(name string) *discordgo.Role {
	r, _ := o.Get(name).Role()
	return r
}

func (o *Options) Attachment(name string) *discordgo.MessageAttachment {
	return o.Get(name).Attachment()
}

type mentionKind uint8

const (
	kindUnknown mentionKind = iota
	kindUser
	kindRole
)

// Value is one parsed argument. Entities resolve lazily (state cache,
// then REST) for prefix invocations; slash uses Discord's resolved data.
type Value struct {
	Type OptionType

	raw  string
	str  string
	num  float64
	b    bool
	id   string
	kind mentionKind

	user       *discordgo.User
	member     *discordgo.Member
	channel    *discordgo.Channel
	role       *discordgo.Role
	attachment *discordgo.MessageAttachment

	s       *discordgo.Session
	guildID string
}

// Raw is the value as typed over prefix, or its string form from slash.
func (v *Value) Raw() string {
	if v == nil {
		return ""
	}
	return v.raw
}

// String is the string value, or Raw() for anything else.
func (v *Value) String() string {
	if v == nil {
		return ""
	}
	if v.Type == OptionString {
		return v.str
	}
	return v.raw
}

func (v *Value) Int() int64 {
	if v == nil {
		return 0
	}
	return int64(v.num)
}

func (v *Value) Float() float64 {
	if v == nil {
		return 0
	}
	return v.num
}

func (v *Value) Bool() bool {
	if v == nil {
		return false
	}
	return v.b
}

func (v *Value) ID() string {
	if v == nil {
		return ""
	}
	return v.id
}

func (v *Value) IsRole() bool { return v != nil && (v.role != nil || v.kind == kindRole) }

func (v *Value) IsUser() bool {
	return v != nil && (v.user != nil || v.member != nil || v.kind == kindUser)
}

func (v *Value) User() (*discordgo.User, error) {
	if v == nil || v.id == "" {
		return nil, ErrMissingOption
	}

	if v.user != nil {
		return v.user, nil
	}

	if v.member != nil && v.member.User != nil {
		return v.member.User, nil
	}

	if v.s == nil || v.kind == kindRole {
		return nil, fmt.Errorf("router: cannot resolve user %s", v.id)
	}

	u, err := v.s.User(v.id)
	if err != nil {
		return nil, err
	}

	v.user = u
	return u, nil
}

func (v *Value) Member() (*discordgo.Member, error) {
	if v == nil || v.id == "" {
		return nil, ErrMissingOption
	}

	if v.member != nil {
		return v.member, nil
	}

	if v.s == nil || v.guildID == "" || v.kind == kindRole {
		return nil, fmt.Errorf("router: cannot resolve member %s", v.id)
	}

	if v.s.State != nil {
		if m, err := v.s.State.Member(v.guildID, v.id); err == nil {
			v.member = m
			return m, nil
		}
	}

	m, err := v.s.GuildMember(v.guildID, v.id)
	if err != nil {
		return nil, err
	}

	v.member = m
	return m, nil
}

func (v *Value) Channel() (*discordgo.Channel, error) {
	if v == nil || v.id == "" {
		return nil, ErrMissingOption
	}

	if v.channel != nil {
		return v.channel, nil
	}

	if v.s == nil {
		return nil, fmt.Errorf("router: cannot resolve channel %s", v.id)
	}

	if v.s.State != nil {
		if ch, err := v.s.State.Channel(v.id); err == nil {
			v.channel = ch
			return ch, nil
		}
	}

	ch, err := v.s.Channel(v.id)
	if err != nil {
		return nil, err
	}

	v.channel = ch
	return ch, nil
}

func (v *Value) Role() (*discordgo.Role, error) {
	if v == nil || v.id == "" {
		return nil, ErrMissingOption
	}

	if v.role != nil {
		return v.role, nil
	}

	if v.s == nil || v.guildID == "" || v.kind == kindUser {
		return nil, fmt.Errorf("router: cannot resolve role %s", v.id)
	}

	if v.s.State != nil {
		if r, err := v.s.State.Role(v.guildID, v.id); err == nil {
			v.role = r
			return r, nil
		}
	}

	roles, err := v.s.GuildRoles(v.guildID)
	if err != nil {
		return nil, err
	}

	for _, r := range roles {
		if r.ID == v.id {
			v.role = r
			return r, nil
		}
	}

	return nil, fmt.Errorf("router: role %s not found", v.id)
}

func (v *Value) Attachment() *discordgo.MessageAttachment {
	if v == nil {
		return nil
	}
	return v.attachment
}

func optionsFromInteraction(
	s *discordgo.Session,
	guildID string,
	opts []*discordgo.ApplicationCommandInteractionDataOption,
	resolved *discordgo.ApplicationCommandInteractionDataResolved,
) *Options {
	out := newOptions()
	for _, opt := range opts {
		if opt == nil {
			continue
		}

		v := &Value{Type: opt.Type, s: s, guildID: guildID}
		switch opt.Type {
		case OptionString:
			v.str, _ = opt.Value.(string)
			v.raw = v.str
		case OptionInteger, OptionNumber:
			v.num = toFloat(opt.Value)
			v.raw = fmt.Sprint(opt.Value)
		case OptionBoolean:
			v.b, _ = opt.Value.(bool)
			v.raw = strconv.FormatBool(v.b)
		default:
			v.id, _ = opt.Value.(string)
			v.raw = v.id
			if resolved != nil {
				v.user = resolved.Users[v.id]
				v.member = resolved.Members[v.id]
				if v.member != nil && v.member.User == nil {
					v.member.User = v.user
				}

				v.channel = resolved.Channels[v.id]
				v.role = resolved.Roles[v.id]
				v.attachment = resolved.Attachments[v.id]
				switch {
				case v.role != nil:
					v.kind = kindRole
				case v.user != nil || v.member != nil:
					v.kind = kindUser
				}
			}
		}

		out.set(opt.Name, v)
	}

	return out
}

func toFloat(x any) float64 {
	switch n := x.(type) {
	case float64:
		return n
	case float32:
		return float64(n)
	case int:
		return float64(n)
	case int64:
		return float64(n)
	case int32:
		return float64(n)
	case string:
		f, _ := strconv.ParseFloat(n, 64)
		return f
	}
	return 0
}
