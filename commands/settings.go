package commands

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/gumi/v2"
	"github.com/disgoorg/disgo/discord"
)

const successColor = 0x3ba55c

func canManage(perms discord.Permissions) bool {
	return perms&(discord.PermissionAdministrator|discord.PermissionManageGuild) != 0
}

// toggle is an on/off setting. One definition drives its /set
// subcommand, its panel button and how it's displayed.
type toggle struct {
	// name is the /set subcommand and the key in panel button IDs.
	name    string
	aliases []string
	label   string
	// short is the panel button label.
	short   string
	hint    string
	section string

	get func(*store.Guild) bool
	set func(*store.Guild, bool)
}

func providerToggle(info artworks.Info) toggle {
	return toggle{
		name: info.Key, aliases: info.Aliases, label: info.Label, short: info.Label,
		hint:    info.Label + " links are handled",
		section: "sources",
		get:     func(g *store.Guild) bool { return g.ProviderEnabled(info.Key) },
		set:     func(g *store.Guild, on bool) { g.SetProvider(info.Key, on) },
	}
}

// postingToggles never change; source toggles are added per provider.
var postingToggles = []toggle{
	{
		name: "tags", label: "Tags", short: "Tags", section: "posting",
		hint: "Show artwork tags in the post",
		get:  func(g *store.Guild) bool { return g.Posting.Tags },
		set:  func(g *store.Guild, on bool) { g.Posting.Tags = on },
	},
	{
		name: "reactions", label: "Reactions", short: "Reactions", section: "posting",
		hint: "Add bookmark reactions to posts",
		get:  func(g *store.Guild) bool { return g.Posting.Reactions },
		set:  func(g *store.Guild, on bool) { g.Posting.Reactions = on },
	},
	{
		name: "skip-first", aliases: []string{"twitter.skip"}, label: "Skip first tweet image", short: "Skip first",
		section: "posting", hint: "Discord already previews it",
		get: func(g *store.Guild) bool { return g.Posting.SkipFirstTweet },
		set: func(g *store.Guild, on bool) { g.Posting.SkipFirstTweet = on },
	},
	{
		name: "crosspost", label: "Crossposting", short: "Crossposting", section: "posting",
		hint: "Let members crosspost artwork from this server",
		get:  func(g *store.Guild) bool { return g.Posting.Crosspost },
		set:  func(g *store.Guild, on bool) { g.Posting.Crosspost = on },
	},
	{
		name: "quotes", aliases: []string{"footer"}, label: "Footer quotes", short: "Footer quotes", section: "posting",
		hint: "A random quote in each post footer",
		get:  func(g *store.Guild) bool { return g.Posting.Quotes },
		set:  func(g *store.Guild, on bool) { g.Posting.Quotes = on },
	},
	{
		name: "nsfw-quotes", aliases: []string{"nsfw"}, label: "NSFW quotes", short: "NSFW quotes", section: "posting",
		hint: "Allow NSFW quotes in post footers",
		get:  func(g *store.Guild) bool { return g.Posting.NSFWQuotes },
		set:  func(g *store.Guild, on bool) { g.Posting.NSFWQuotes = on },
	},
}

// settingsSpec holds all toggles: the posting ones plus one for each
// registered provider.
type settingsSpec struct {
	toggles []toggle
}

func newSettingsSpec(providers []artworks.Provider) *settingsSpec {
	spec := &settingsSpec{toggles: slices.Clone(postingToggles)}
	for _, p := range providers {
		spec.toggles = append(spec.toggles, providerToggle(p.Info()))
	}
	return spec
}

func (s *settingsSpec) find(name string) (toggle, bool) {
	for _, t := range s.toggles {
		if t.name == name {
			return t, true
		}
	}
	return toggle{}, false
}

func (s *settingsSpec) in(section string) []toggle {
	var out []toggle
	for _, t := range s.toggles {
		if t.section == section {
			out = append(out, t)
		}
	}
	return out
}

// sourceLabels returns the provider names, comma separated.
func (s *settingsSpec) sourceLabels() string {
	labels := make([]string, 0, 4)
	for _, t := range s.in("sources") {
		labels = append(labels, t.label)
	}
	return strings.Join(labels, ", ")
}

type repostMode struct {
	mode  store.RepostMode
	label string
	hint  string
}

var repostModes = []repostMode{
	{store.RepostOff, "Off", "Don't check for reposts"},
	{store.RepostNotify, "Notify", "Reply with a notice when a link was already posted"},
	{store.RepostStrict, "Strict", "Delete reposts and notify the poster"},
}

func findRepostMode(m store.RepostMode) (repostMode, bool) {
	for _, rm := range repostModes {
		if rm.mode == m {
			return rm, true
		}
	}
	return repostMode{}, false
}

type ttlPreset struct {
	name, value string
	ttl         time.Duration
}

var ttlPresets = []ttlPreset{
	{"1 hour", "1h", time.Hour},
	{"6 hours", "6h", 6 * time.Hour},
	{"12 hours", "12h", 12 * time.Hour},
	{"1 day", "1d", 24 * time.Hour},
	{"3 days", "3d", 72 * time.Hour},
	{"7 days", "7d", store.MaxRepostTTL},
}

func findTTL(value string) (time.Duration, bool) {
	for _, p := range ttlPresets {
		if p.value == value {
			return p.ttl, true
		}
	}
	return 0, false
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

func allowed(on bool) string {
	if on {
		return "allowed"
	}
	return "blocked"
}

// formatTTL renders a repost TTL as days, hours or minutes.
func formatTTL(d time.Duration) string {
	plural := func(n int, unit string) string {
		if n == 1 {
			return "1 " + unit
		}
		return strconv.Itoa(n) + " " + unit + "s"
	}

	switch {
	case d >= 24*time.Hour && d%(24*time.Hour) == 0:
		return plural(int(d/(24*time.Hour)), "day")
	case d >= time.Hour && d%time.Hour == 0:
		return plural(int(d/time.Hour), "hour")
	case d%time.Minute == 0:
		return plural(int(d/time.Minute), "minute")
	default:
		return d.String()
	}
}

// settingChange is what a change did, for the confirmation reply.
type settingChange struct {
	name, old, new, hint string
}

func (c settingChange) embed() discord.Embed {
	desc := fmt.Sprintf("**%s** %s → **%s**", c.name, c.old, c.new)
	if c.hint != "" {
		desc += "\n-# " + c.hint
	}

	return discord.Embed{Color: successColor, Description: desc}
}

func applyPrefix(g *store.Guild, value string) (settingChange, error) {
	prefix := strings.TrimSpace(value)

	// A trailing letter would glue the prefix to the command name.
	if r := []rune(prefix); len(r) > 0 && unicode.IsLetter(r[len(r)-1]) {
		prefix += " "
	}

	if n := len([]rune(prefix)); n < 1 || n > store.MaxPrefixLength {
		return settingChange{}, gumi.Errorf("Prefixes are 1 to %d characters long, counting the space added after letters.", store.MaxPrefixLength)
	}

	c := settingChange{name: "Prefix", old: "`" + g.Prefix + "`", new: "`" + prefix + "`", hint: "Slash commands and mentions always work"}
	g.Prefix = prefix

	return c, nil
}

func applyLimit(g *store.Guild, limit int) (settingChange, error) {
	if limit < store.MinPostLimit || limit > store.MaxPostLimit {
		return settingChange{}, gumi.Errorf("Images per post must be between %d and %d.", store.MinPostLimit, store.MaxPostLimit)
	}

	c := settingChange{name: "Images per post", old: strconv.Itoa(g.Posting.Limit), new: strconv.Itoa(limit), hint: "Longer galleries are cut off after this many"}
	g.Posting.Limit = limit

	return c, nil
}

func applyToggle(g *store.Guild, t toggle, on bool) settingChange {
	c := settingChange{name: t.label, old: onOff(t.get(g)), new: onOff(on), hint: t.hint}
	t.set(g, on)

	return c
}

// applyReposts changes the mode and/or TTL; empty values keep the current one.
func applyReposts(g *store.Guild, mode, ttl string) (settingChange, error) {
	before := g.Repost

	if mode != "" {
		if _, ok := findRepostMode(store.RepostMode(mode)); !ok {
			return settingChange{}, gumi.Errorf("Unknown repost mode `%s`.", mode)
		}
		g.Repost.Mode = store.RepostMode(mode)
	}

	if ttl != "" {
		d, ok := findTTL(ttl)
		if !ok {
			return settingChange{}, gumi.Errorf("Unknown repost time `%s`.", ttl)
		}
		g.Repost.TTL = d
	}

	describe := func(r store.Repost) string {
		m, _ := findRepostMode(r.Mode)
		return m.label + ", " + formatTTL(r.TTL)
	}

	m, _ := findRepostMode(g.Repost.Mode)

	return settingChange{name: "Reposts", old: describe(before), new: describe(g.Repost), hint: m.hint}, nil
}

func settingsGroup(b *bot.Bot) []*gumi.Command {
	spec := newSettingsSpec(b.ArtworkProviders)
	panel := &settingsPanel{b: b, spec: spec}
	panel.cmd = &gumi.Command{
		Name:        "settings",
		Category:    "Settings",
		Aliases:     []string{"cfg", "config"},
		Description: "Shows server settings with controls to change them.",
		Checks:      []gumi.Check{gumi.GuildOnly},
		Cooldown:    gumi.NewCooldown(gumi.CooldownUser, 1, 5*time.Second),
		Ephemeral:   true,
		Examples:    []string{"settings"},
		Handler:     panel.open,
		Components:  panel.handle,
	}

	return []*gumi.Command{panel.cmd, setCommand(b, spec), channelsCommand(b)}
}

func setCommand(b *bot.Bot, spec *settingsSpec) *gumi.Command {
	modeChoices := make([]gumi.Choice, 0, len(repostModes))
	for _, m := range repostModes {
		modeChoices = append(modeChoices, gumi.Choice{Name: m.label, Value: string(m.mode)})
	}

	ttlChoices := make([]gumi.Choice, 0, len(ttlPresets))
	for _, p := range ttlPresets {
		ttlChoices = append(ttlChoices, gumi.Choice{Name: p.name, Value: p.value})
	}

	subs := []*gumi.Command{
		{
			Name:        "prefix",
			Description: "Changes the prefix for prefix commands.",
			Options: []*gumi.Option{
				gumi.String("value", "New prefix, up to 5 characters").Require().WithLength(1, store.MaxPrefixLength),
			},
			Examples: []string{"set prefix !"},
			Handler: func(ctx *gumi.Context) error {
				return changeGuild(ctx, b, func(g *store.Guild) (settingChange, error) {
					return applyPrefix(g, ctx.Options.String("value"))
				})
			},
		},
		{
			Name:        "limit",
			Description: "Changes how many images a post shows.",
			Options: []*gumi.Option{
				gumi.Integer("value", "Images per post").Require().WithRange(store.MinPostLimit, store.MaxPostLimit),
			},
			Examples: []string{"set limit 20"},
			Handler: func(ctx *gumi.Context) error {
				return changeGuild(ctx, b, func(g *store.Guild) (settingChange, error) {
					return applyLimit(g, int(ctx.Options.Int("value")))
				})
			},
		},
		{
			Name:        "reposts",
			Aliases:     []string{"repost"},
			Description: "Changes repost detection.",
			Options: []*gumi.Option{
				gumi.String("mode", "What to do with reposts").Require().WithChoices(modeChoices...),
				gumi.String("remember", "How long links count towards reposts").WithChoices(ttlChoices...),
			},
			Examples: []string{"set reposts strict 3d"},
			Handler: func(ctx *gumi.Context) error {
				return changeGuild(ctx, b, func(g *store.Guild) (settingChange, error) {
					return applyReposts(g, ctx.Options.String("mode"), ctx.Options.String("remember"))
				})
			},
		},
	}

	for _, t := range spec.toggles {
		subs = append(subs, &gumi.Command{
			Name:        t.name,
			Aliases:     t.aliases,
			Description: "Turns " + strings.ToLower(t.label[:1]) + t.label[1:] + " on or off.",
			Options:     []*gumi.Option{gumi.Boolean("enabled", "On or off").Require()},
			Examples:    []string{"set " + t.name + " off"},
			Handler: func(ctx *gumi.Context) error {
				return changeGuild(ctx, b, func(g *store.Guild) (settingChange, error) {
					return applyToggle(g, t, ctx.Options.Bool("enabled")), nil
				})
			},
		})
	}

	return &gumi.Command{
		Name:        "set",
		Category:    "Settings",
		Description: "Changes one server setting.",
		Checks:      []gumi.Check{gumi.GuildOnly, gumi.HasPermissions(discord.PermissionManageGuild)},
		Cooldown:    gumi.NewCooldown(gumi.CooldownUser, 1, 5*time.Second),
		Subcommands: subs,
	}
}

// changeGuild applies change to the invoking guild, saves it and confirms.
func changeGuild(ctx *gumi.Context, b *bot.Bot, change func(*store.Guild) (settingChange, error)) error {
	reqCtx, cancel := context.WithTimeout(ctx.Context(), 10*time.Second)
	defer cancel()

	guild, _, err := store.GetOrCreateGuild(reqCtx, b.Store, ctx.GuildID().String())
	if err != nil {
		return err
	}

	c, err := change(guild)
	if err != nil {
		return err
	}

	if _, err := b.Store.UpdateGuild(reqCtx, guild); err != nil {
		return err
	}

	return ctx.ReplyEmbed(c.embed())
}
