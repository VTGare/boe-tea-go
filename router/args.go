package router

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	userMentionRe    = regexp.MustCompile(`^<@!?(\d+)>$`)
	channelMentionRe = regexp.MustCompile(`^<#(\d+)>$`)
	roleMentionRe    = regexp.MustCompile(`^<@&(\d+)>$`)
	snowflakeRe      = regexp.MustCompile(`^\d{15,22}$`)
	messageLinkRe    = regexp.MustCompile(`^https?://(?:[\w-]+\.)?discord(?:app)?\.com/channels/(\d+|@me)/(\d+)/(\d+)/?$`)
)

// token is a single whitespace-separated (or quoted) argument with its byte
// offsets in the source string.
type token struct {
	text  string
	start int
	end   int
}

// tokenize splits s on ASCII whitespace, honouring "double" and 'single'
// quotes. Unterminated quotes are treated literally.
func tokenize(s string) []token {
	var toks []token
	n := len(s)
	i := 0

	for i < n {
		for i < n && isSpace(s[i]) {
			i++
		}
		if i >= n {
			break
		}
		start := i
		if q := s[i]; q == '"' || q == '\'' {
			if j := strings.IndexByte(s[i+1:], q); j >= 0 {
				end := i + 1 + j
				toks = append(toks, token{text: s[i+1 : end], start: start, end: end + 1})
				i = end + 1
				continue
			}
		}
		j := i
		for j < n && !isSpace(s[j]) {
			j++
		}
		toks = append(toks, token{text: s[i:j], start: start, end: j})
		i = j
	}
	return toks
}

func isSpace(b byte) bool {
	return b == ' ' || b == '\t' || b == '\n' || b == '\r'
}

func parsePrefixOptions(ctx *Context, cmd *Command, raw string) (*Options, error) {
	out := newOptions()
	toks := tokenize(raw)
	ti := 0
	attachmentIdx := 0

	for _, o := range cmd.Options {
		if o.NoPrefix {
			continue
		}

		if o.Type == OptionAttachment {
			atts := ctx.messageAttachments()
			if attachmentIdx < len(atts) {
				a := atts[attachmentIdx]
				attachmentIdx++
				out.set(o.Name, &Value{Type: o.Type, raw: a.URL, id: a.ID, attachment: a, s: ctx.Session, guildID: ctx.GuildID()})
			} else if o.Required {
				return out, &OptionError{Option: o, Err: ErrMissingOption}
			}
			continue
		}

		if ti >= len(toks) {
			if o.Required {
				return out, &OptionError{Option: o, Err: ErrMissingOption}
			}
			continue
		}

		var text string
		if o.Rest {
			text = strings.TrimSpace(raw[toks[ti].start:])
			ti = len(toks)
		} else {
			text = toks[ti].text
			ti++
		}

		v, err := parseValue(ctx, o, text)
		if err != nil {
			return out, &OptionError{Option: o, Value: text, Err: err}
		}

		out.set(o.Name, v)
	}

	return out, nil
}

func parseValue(ctx *Context, o *Option, text string) (*Value, error) {
	v := &Value{Type: o.Type, raw: text, s: ctx.Session, guildID: ctx.GuildID()}

	if len(o.Choices) > 0 && hasChoices(o.Type) {
		return parseChoice(v, o, text)
	}

	var err error
	switch o.Type {
	case OptionString:
		v.str, err = text, checkLength(o, text)
	case OptionInteger:
		v.num, err = parseNumber(o, text, "expected a whole number", func(s string) (float64, error) {
			n, err := strconv.ParseInt(s, 10, 64)
			return float64(n), err
		})
	case OptionNumber:
		v.num, err = parseNumber(o, text, "expected a number", func(s string) (float64, error) {
			return strconv.ParseFloat(s, 64)
		})
	case OptionBoolean:
		var ok bool
		if v.b, ok = parseBool(text); !ok {
			err = errors.New("expected yes/no, on/off or true/false")
		}
	case OptionUser:
		v.id, v.kind = parseMention(text, userMentionRe), kindUser
		err = requireID(v.id, "expected a user mention or ID")
	case OptionRole:
		v.id, v.kind = parseMention(text, roleMentionRe), kindRole
		err = requireID(v.id, "expected a role mention or ID")
	case OptionChannel:
		v.id = parseMention(text, channelMentionRe)
		if err = requireID(v.id, "expected a channel mention or ID"); err == nil {
			err = checkChannelType(o, v)
		}
	case OptionMentionable:
		v.id, v.kind = parseMentionable(text)
		err = requireID(v.id, "expected a user or role mention")
	default:
		err = fmt.Errorf("unsupported option type %d", o.Type)
	}

	if err != nil {
		return nil, err
	}

	return v, nil
}

// hasChoices reports whether Discord allows choices on an option type.
func hasChoices(t OptionType) bool {
	return t == OptionString || t == OptionInteger || t == OptionNumber
}

// parseChoice maps typed text onto a declared choice; its value, not the
// text, is then validated.
func parseChoice(v *Value, o *Option, text string) (*Value, error) {
	c, ok := matchChoice(o, text)
	if !ok {
		return nil, choiceError(o)
	}

	var err error
	if o.Type == OptionString {
		v.str = fmt.Sprint(c.Value)
		err = checkLength(o, v.str)
	} else {
		v.num = toFloat(c.Value)
		err = checkRange(o, v.num)
	}

	if err != nil {
		return nil, err
	}

	return v, nil
}

func parseNumber(o *Option, text, invalid string, parse func(string) (float64, error)) (float64, error) {
	n, err := parse(text)
	if err != nil {
		return 0, errors.New(invalid)
	}

	return n, checkRange(o, n)
}

func checkLength(o *Option, s string) error {
	n := utf8.RuneCountInString(s)
	if o.MinLength != nil && n < *o.MinLength {
		return fmt.Errorf("expected at least %d characters", *o.MinLength)
	}

	if o.MaxLength != nil && n > *o.MaxLength {
		return fmt.Errorf("expected at most %d characters", *o.MaxLength)
	}

	return nil
}

func checkChannelType(o *Option, v *Value) error {
	if len(o.ChannelTypes) == 0 {
		return nil
	}

	ch, err := v.Channel()
	if err != nil {
		return errors.New("unknown channel")
	}

	if !slices.Contains(o.ChannelTypes, ch.Type) {
		return errors.New("this channel type is not allowed here")
	}

	return nil
}

func requireID(id, invalid string) error {
	if id == "" {
		return errors.New(invalid)
	}

	return nil
}

// parseMentionable resolves a user mention, role mention or bare ID.
func parseMentionable(text string) (string, mentionKind) {
	if m := userMentionRe.FindStringSubmatch(text); m != nil {
		return m[1], kindUser
	}

	if m := roleMentionRe.FindStringSubmatch(text); m != nil {
		return m[1], kindRole
	}

	if snowflakeRe.MatchString(text) {
		return text, kindUnknown
	}

	return "", kindUnknown
}

func checkRange(o *Option, n float64) error {
	if o.MinValue != nil && n < *o.MinValue {
		return fmt.Errorf("expected a value of at least %v", *o.MinValue)
	}

	if o.MaxValue != nil && n > *o.MaxValue {
		return fmt.Errorf("expected a value of at most %v", *o.MaxValue)
	}

	return nil
}

func matchChoice(o *Option, text string) (Choice, bool) {
	for _, c := range o.Choices {
		if strings.EqualFold(c.Name, text) || strings.EqualFold(fmt.Sprint(c.Value), text) {
			return c, true
		}
	}
	return Choice{}, false
}

func choiceError(o *Option) error {
	names := make([]string, 0, len(o.Choices))
	for _, c := range o.Choices {
		names = append(names, c.Name)
	}
	return fmt.Errorf("expected one of %s", strings.Join(names, ", "))
}

func parseBool(text string) (bool, bool) {
	switch strings.ToLower(text) {
	case "true", "yes", "y", "on", "1", "enable", "enabled":
		return true, true
	case "false", "no", "n", "off", "0", "disable", "disabled":
		return false, true
	}
	return false, false
}

// parseMention extracts a snowflake from a mention matching re or a bare ID.
func parseMention(text string, re *regexp.Regexp) string {
	if m := re.FindStringSubmatch(text); m != nil {
		return m[1]
	}

	if snowflakeRe.MatchString(text) {
		return text
	}

	return ""
}

// parseMessageRef extracts (channelID, messageID) from a message link or a
// bare message ID (which is assumed to be in fallbackChannel).
func parseMessageRef(text, fallbackChannel string) (channelID, messageID string, ok bool) {
	if m := messageLinkRe.FindStringSubmatch(text); m != nil {
		return m[2], m[3], true
	}

	if snowflakeRe.MatchString(text) {
		return fallbackChannel, text, true
	}

	return "", "", false
}
