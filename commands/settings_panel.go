package commands

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/VTGare/gumi/v2"
	disgobot "github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
)

// settingsPanel is the /settings message: an overview plus a page per
// section. All state lives in the button IDs (owner, action, arg), so
// the panel keeps working across restarts.
type settingsPanel struct {
	b    *bot.Bot
	cmd  *gumi.Command
	spec *settingsSpec
}

type panelSection struct {
	key, title, summary string
}

func (s *settingsSpec) sections() []panelSection {
	return []panelSection{
		{"general", "General", "Prefix"},
		{"posting", "Posting", "Images per post, tags, reactions, footer quotes"},
		{"sources", "Sources", s.sourceLabels()},
		{"reposts", "Reposts", "Repost detection and how long links are remembered"},
		{"channels", "Art channels", "Where Boe Tea posts artwork"},
	}
}

// maxListedChannels caps channel mentions in the panel.
const maxListedChannels = 30

func (p *settingsPanel) open(ctx *gumi.Context) error {
	reqCtx, cancel := context.WithTimeout(ctx.Context(), 10*time.Second)
	defer cancel()

	guild, _, err := store.GetOrCreateGuild(reqCtx, p.b.Store, ctx.GuildID().String())
	if err != nil {
		return err
	}

	return ctx.Reply(p.view(ctx.Client, guild, ctx.AuthorID().String()).render("home"))
}

// handle handles clicks on the panel. Only the person who opened it can
// use it, and changing anything needs Manage Server.
func (p *settingsPanel) handle(ctx *gumi.ComponentContext) error {
	owner, action, arg := ctx.Arg(0), ctx.Arg(1), ctx.Arg(2)

	if ctx.UserID().String() != owner {
		return ctx.Reply(gumi.Text("This panel belongs to someone else. Run `/settings` for your own.").Private())
	}

	navigating := action == "go" || action == "nav"
	if !navigating && !canManage(ctx.Permissions()) {
		return ctx.Reply(gumi.Text("You need the Manage Server permission to change settings.").Private())
	}

	if action == "edit" {
		return p.openModal(ctx, owner, arg)
	}

	reqCtx, cancel := context.WithTimeout(p.b.Context, 10*time.Second)
	defer cancel()

	guild, _, err := store.GetOrCreateGuild(reqCtx, p.b.Store, ctx.GuildID().String())
	if err != nil {
		return err
	}

	section, err := p.apply(reqCtx, ctx, guild, action, arg)
	if err != nil {
		return err
	}

	return ctx.Update(p.view(ctx.Client, guild, owner).render(section))
}

// apply runs one panel action, changing guild in place, and returns the
// section to show next.
func (p *settingsPanel) apply(reqCtx context.Context, ctx *gumi.ComponentContext, guild *store.Guild, action, arg string) (string, error) {
	value := ""
	if values := ctx.Values(); len(values) > 0 {
		value = values[0]
	}

	save := func(err error) error {
		if err != nil {
			return err
		}
		_, err = p.b.Store.UpdateGuild(reqCtx, guild)
		return err
	}

	switch action {
	case "go":
		return arg, nil
	case "nav":
		return value, nil
	case "toggle":
		t, ok := p.spec.find(arg)
		if !ok {
			return "home", nil
		}
		applyToggle(guild, t, !t.get(guild))
		return t.section, save(nil)
	case "mode":
		_, err := applyReposts(guild, value, "")
		return "reposts", save(err)
	case "ttl":
		_, err := applyReposts(guild, "", value)
		return "reposts", save(err)
	case "submit":
		input := ctx.TextInput("value")
		if arg == "prefix" {
			_, err := applyPrefix(guild, input)
			return "general", save(err)
		}

		limit, err := strconv.Atoi(strings.TrimSpace(input))
		if err != nil {
			return "", gumi.Errorf("Images per post must be a whole number from %d to %d.", store.MinPostLimit, store.MaxPostLimit)
		}
		_, err = applyLimit(guild, limit)
		return "posting", save(err)
	case "add", "remove":
		all, err := guildChannels(ctx.Client, ctx.GuildID())
		if err != nil {
			return "", err
		}

		ids := expandArtChannels(all, pickedChannels(all, ctx.Values()))

		var updated *store.Guild
		if action == "add" {
			updated, err = p.b.Store.AddArtChannels(reqCtx, guild.ID, ids)
		} else {
			updated, err = p.b.Store.DeleteArtChannels(reqCtx, guild.ID, ids)
		}
		if err != nil {
			return "", err
		}

		*guild = *updated
		return "channels", nil
	}

	return "home", nil
}

func (p *settingsPanel) openModal(ctx *gumi.ComponentContext, owner, field string) error {
	id := gumi.ComponentID(p.cmd, owner, "submit", field)

	if field == "prefix" {
		return ctx.Modal(id, "Change prefix", discord.NewLabel("Prefix (1-5 characters)",
			discord.NewShortTextInput("value").WithRequired(true).WithMinLength(1).WithMaxLength(store.MaxPrefixLength),
		))
	}

	return ctx.Modal(id, "Images per post", discord.NewLabel(fmt.Sprintf("Images per post (%d-%d)", store.MinPostLimit, store.MaxPostLimit),
		discord.NewShortTextInput("value").WithRequired(true).WithMinLength(1).WithMaxLength(3),
	))
}

func (p *settingsPanel) view(c *disgobot.Client, guild *store.Guild, owner string) *panelView {
	v := &panelView{cmd: p.cmd, spec: p.spec, owner: owner, guild: guild}

	id := dgoutils.ParseID(guild.ID)

	g, ok := c.Caches.Guild(id)
	if !ok {
		if rg, err := c.Rest.GetGuild(id, false); err == nil {
			g, ok = rg.Guild, true
		}
	}

	if ok {
		v.name = g.Name
		if icon := g.IconURL(discord.WithSize(128)); icon != nil {
			v.icon = *icon
		}
	}

	return v
}

// panelView renders the panel for one guild.
type panelView struct {
	cmd        *gumi.Command
	spec       *settingsSpec
	owner      string
	guild      *store.Guild
	name, icon string
}

func (v *panelView) id(action string, arg ...string) string {
	return gumi.ComponentID(v.cmd, append([]string{v.owner, action}, arg...)...)
}

func (v *panelView) render(section string) *gumi.Response {
	embed := discord.Embed{Color: 0x439ef1}
	if v.name != "" {
		embed.Author = &discord.EmbedAuthor{Name: v.name, IconURL: v.icon}
	}

	var rows []discord.LayoutComponent
	back := button("Back to settings", discord.ButtonStyleSecondary, v.id("go", "home"))

	switch section {
	case "general":
		embed.Title = "General"
		embed.Description = "Prefix commands start with this. Slash commands and mentioning the bot always work.\n\n" +
			line("Prefix", "`"+v.guild.Prefix+"`", fmt.Sprintf("Up to %d characters, e.g. bt! or ?", store.MaxPrefixLength))
		rows = buttonRows(button("Change prefix", discord.ButtonStyleSecondary, v.id("edit", "prefix")), back)
	case "posting":
		embed.Title = "Posting"
		lines := []string{"How Boe Tea posts artwork from links.", line("Images per post", strconv.Itoa(v.guild.Posting.Limit), "Longer galleries are cut off after this many")}
		buttons := make([]discord.InteractiveComponent, 0, 8)
		for _, t := range v.spec.in("posting") {
			lines = append(lines, line(t.label, onOff(t.get(v.guild)), t.hint))
			buttons = append(buttons, v.toggleButton(t))
		}
		embed.Description = strings.Join(lines, "\n\n")
		buttons = append(buttons, button(fmt.Sprintf("Images per post: %d", v.guild.Posting.Limit), discord.ButtonStyleSecondary, v.id("edit", "limit")))
		rows = append(buttonRows(buttons...), buttonRows(back)...)
	case "sources":
		embed.Title = "Sources"
		embed.Description = "Links from turned-off sources are ignored."
		buttons := make([]discord.InteractiveComponent, 0, 4)
		for _, t := range v.spec.in("sources") {
			buttons = append(buttons, v.toggleButton(t))
		}
		rows = append(buttonRows(buttons...), buttonRows(back)...)
	case "reposts":
		mode, _ := findRepostMode(v.guild.Repost.Mode)
		embed.Title = "Reposts"
		embed.Description = "Catch artwork that was already posted in this server.\n\n" +
			line("Mode", mode.label, mode.hint) + "\n\n" +
			line("Remember links for", formatTTL(v.guild.Repost.TTL), "A link counts as a repost within this time")
		rows = []discord.LayoutComponent{v.modeSelect(), v.ttlSelect()}
		rows = append(rows, buttonRows(back)...)
	case "channels":
		embed.Title = fmt.Sprintf("Art channels · %d", len(v.guild.ArtChannels))
		embed.Description = channelsSummary(v.guild.ArtChannels, maxListedChannels)
		rows = []discord.LayoutComponent{
			v.channelSelect("add", "Add channels or categories…"),
			v.channelSelect("remove", "Remove channels or categories…"),
		}
		rows = append(rows, buttonRows(back)...)
	default:
		v.renderHome(&embed)
		rows = []discord.LayoutComponent{v.navSelect()}
	}

	return &gumi.Response{Embeds: []discord.Embed{embed}, Components: compact(rows)}
}

func (v *panelView) renderHome(embed *discord.Embed) {
	g := v.guild
	p := g.Posting
	mode, _ := findRepostMode(g.Repost.Mode)

	reposts := "**" + mode.label + "** · links are remembered for " + formatTTL(g.Repost.TTL)
	if g.Repost.Mode == store.RepostOff {
		reposts = "**Off**\n-# When on, links are remembered for " + formatTTL(g.Repost.TTL)
	}

	sources := make([]string, 0, 4)
	for _, t := range v.spec.in("sources") {
		sources = append(sources, t.label+" **"+onOff(t.get(g))+"**")
	}

	embed.Title = "Settings"
	embed.Description = "Pick a section below to change it. Quick edits: `/set` and `/channels`."
	embed.Fields = []discord.EmbedField{
		{Name: "General", Value: "Prefix `" + g.Prefix + "`"},
		{Name: "Posting", Value: fmt.Sprintf(
			"Up to **%d** images per post · Tags **%s** · Reactions **%s**\nSkip first tweet image **%s** · Crossposting **%s**\nFooter quotes **%s** · NSFW quotes **%s**",
			p.Limit, onOff(p.Tags), onOff(p.Reactions), onOff(p.SkipFirstTweet), onOff(p.Crosspost), onOff(p.Quotes), allowed(p.NSFWQuotes),
		)},
		{Name: "Sources", Value: strings.Join(sources, " · ")},
		{Name: "Reposts", Value: reposts},
		{Name: fmt.Sprintf("Art channels · %d", len(g.ArtChannels)), Value: channelsSummary(g.ArtChannels, 15)},
	}
	embed.Footer = &discord.EmbedFooter{Text: "Changing settings needs Manage Server"}
	if v.icon != "" {
		embed.Thumbnail = &discord.EmbedResource{URL: v.icon}
	}
}

// line formats a setting: bold name and value, then a small hint line.
func line(name, value, hint string) string {
	return fmt.Sprintf("**%s** %s\n-# %s", name, value, hint)
}

func channelsSummary(ids []string, limit int) string {
	if len(ids) == 0 {
		return "None: Boe Tea posts in every channel."
	}

	mentions := make([]string, 0, min(len(ids), limit))
	for _, id := range ids[:min(len(ids), limit)] {
		mentions = append(mentions, "<#"+id+">")
	}

	s := strings.Join(mentions, " ")
	if extra := len(ids) - limit; extra > 0 {
		s += fmt.Sprintf(" and %d more", extra)
	}

	return s + "\n-# Boe Tea only posts artwork in these channels"
}

// button is nil when the custom ID is too long to build.
func button(label string, style discord.ButtonStyle, id string) discord.InteractiveComponent {
	if id == "" {
		return nil
	}
	return discord.NewButton(style, label, id, "", 0)
}

func (v *panelView) toggleButton(t toggle) discord.InteractiveComponent {
	if t.get(v.guild) {
		return button(t.short+": On", discord.ButtonStyleSuccess, v.id("toggle", t.name))
	}
	return button(t.short+": Off", discord.ButtonStyleSecondary, v.id("toggle", t.name))
}

func (v *panelView) navSelect() discord.LayoutComponent {
	sections := v.spec.sections()
	options := make([]discord.StringSelectMenuOption, 0, len(sections))
	for _, s := range sections {
		options = append(options, discord.StringSelectMenuOption{Label: s.title, Value: s.key, Description: s.summary})
	}
	return v.stringSelect("nav", "Edit a section…", options)
}

func (v *panelView) modeSelect() discord.LayoutComponent {
	options := make([]discord.StringSelectMenuOption, 0, len(repostModes))
	for _, m := range repostModes {
		options = append(options, discord.StringSelectMenuOption{
			Label: "Mode: " + m.label, Value: string(m.mode), Description: m.hint, Default: m.mode == v.guild.Repost.Mode,
		})
	}
	return v.stringSelect("mode", "Repost mode", options)
}

func (v *panelView) ttlSelect() discord.LayoutComponent {
	options := make([]discord.StringSelectMenuOption, 0, len(ttlPresets))
	for _, p := range ttlPresets {
		options = append(options, discord.StringSelectMenuOption{
			Label: "Remember links for " + p.name, Value: p.value, Default: p.ttl == v.guild.Repost.TTL,
		})
	}
	return v.stringSelect("ttl", "Remember links for…", options)
}

func (v *panelView) stringSelect(action, placeholder string, options []discord.StringSelectMenuOption) discord.LayoutComponent {
	id := v.id(action)
	if id == "" {
		return nil
	}

	return discord.NewActionRow(discord.StringSelectMenuComponent{
		CustomID: id, Placeholder: placeholder, Options: options,
	})
}

func (v *panelView) channelSelect(action, placeholder string) discord.LayoutComponent {
	id := v.id(action)
	if id == "" {
		return nil
	}

	return discord.NewActionRow(discord.ChannelSelectMenuComponent{
		CustomID:     id,
		Placeholder:  placeholder,
		MaxValues:    25,
		ChannelTypes: append(slices.Clone(artChannelTypes), discord.ChannelTypeGuildCategory),
	})
}

// buttonRows packs buttons into rows of Discord's maximum of five.
func buttonRows(buttons ...discord.InteractiveComponent) []discord.LayoutComponent {
	var rows []discord.LayoutComponent
	row := make([]discord.InteractiveComponent, 0, 5)
	for _, b := range buttons {
		if b == nil {
			continue
		}
		if len(row) == 5 {
			rows = append(rows, discord.NewActionRow(row...))
			row = make([]discord.InteractiveComponent, 0, 5)
		}
		row = append(row, b)
	}
	if len(row) > 0 {
		rows = append(rows, discord.NewActionRow(row...))
	}
	return rows
}

func compact(rows []discord.LayoutComponent) []discord.LayoutComponent {
	out := make([]discord.LayoutComponent, 0, len(rows))
	for _, r := range rows {
		if r != nil {
			out = append(out, r)
		}
	}
	return out
}
