package commands

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/router"
	"github.com/VTGare/boe-tea-go/store"
	"github.com/bwmarrin/discordgo"
)

// settingsPanel is the /settings message: an overview plus a page per
// section. All state lives in the button IDs (owner, action, arg), so
// the panel keeps working across restarts.
type settingsPanel struct {
	b    *bot.Bot
	cmd  *router.Command
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

func (p *settingsPanel) open(ctx *router.Context) error {
	reqCtx, cancel := context.WithTimeout(ctx.Context(), 10*time.Second)
	defer cancel()

	guild, _, err := store.GetOrCreateGuild(reqCtx, p.b.Store, ctx.GuildID())
	if err != nil {
		return err
	}

	return ctx.Reply(p.view(ctx.Session, guild, ctx.AuthorID()).render("home"))
}

// handle handles clicks on the panel. Only the person who opened it can
// use it, and changing anything needs Manage Server.
func (p *settingsPanel) handle(ctx *router.ComponentContext) error {
	owner, action, arg := ctx.Arg(0), ctx.Arg(1), ctx.Arg(2)

	if ctx.UserID() != owner {
		return ctx.Reply(router.Text("This panel belongs to someone else. Run `/settings` for your own.").Private())
	}

	navigating := action == "go" || action == "nav"
	if !navigating && !canManage(ctx.Permissions()) {
		return ctx.Reply(router.Text("You need the Manage Server permission to change settings.").Private())
	}

	if action == "edit" {
		return p.openModal(ctx, owner, arg)
	}

	reqCtx, cancel := context.WithTimeout(p.b.Context, 10*time.Second)
	defer cancel()

	guild, _, err := store.GetOrCreateGuild(reqCtx, p.b.Store, ctx.GuildID())
	if err != nil {
		return err
	}

	section, err := p.apply(reqCtx, ctx, guild, action, arg)
	if err != nil {
		return err
	}

	return ctx.Update(p.view(ctx.Session, guild, owner).render(section))
}

// apply runs one panel action, changing guild in place, and returns the
// section to show next.
func (p *settingsPanel) apply(reqCtx context.Context, ctx *router.ComponentContext, guild *store.Guild, action, arg string) (string, error) {
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
			return "", router.Errorf("Images per post must be a whole number from %d to %d.", store.MinPostLimit, store.MaxPostLimit)
		}
		_, err = applyLimit(guild, limit)
		return "posting", save(err)
	case "add", "remove":
		all, err := guildChannels(ctx.Session, ctx.GuildID())
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

func (p *settingsPanel) openModal(ctx *router.ComponentContext, owner, field string) error {
	id := router.ComponentID(p.cmd, owner, "submit", field)

	if field == "prefix" {
		return ctx.Modal(id, "Change prefix", discordgo.TextInput{
			CustomID: "value", Label: "Prefix (1-5 characters)", Style: discordgo.TextInputShort,
			Required: true, MinLength: 1, MaxLength: store.MaxPrefixLength,
		})
	}

	return ctx.Modal(id, "Images per post", discordgo.TextInput{
		CustomID: "value", Label: fmt.Sprintf("Images per post (%d-%d)", store.MinPostLimit, store.MaxPostLimit),
		Style: discordgo.TextInputShort, Required: true, MinLength: 1, MaxLength: 3,
	})
}

func (p *settingsPanel) view(s *discordgo.Session, guild *store.Guild, owner string) *panelView {
	v := &panelView{cmd: p.cmd, spec: p.spec, owner: owner, guild: guild}

	var g *discordgo.Guild
	if s.State != nil {
		g, _ = s.State.Guild(guild.ID)
	}
	if g == nil {
		g, _ = s.Guild(guild.ID)
	}
	if g != nil {
		v.name, v.icon = g.Name, g.IconURL("128")
	}

	return v
}

// panelView renders the panel for one guild.
type panelView struct {
	cmd        *router.Command
	spec       *settingsSpec
	owner      string
	guild      *store.Guild
	name, icon string
}

func (v *panelView) id(action string, arg ...string) string {
	return router.ComponentID(v.cmd, append([]string{v.owner, action}, arg...)...)
}

func (v *panelView) render(section string) *router.Response {
	embed := &discordgo.MessageEmbed{Color: 0x439ef1}
	if v.name != "" {
		embed.Author = &discordgo.MessageEmbedAuthor{Name: v.name, IconURL: v.icon}
	}

	var rows []discordgo.MessageComponent
	back := button("Back to settings", discordgo.SecondaryButton, v.id("go", "home"))

	switch section {
	case "general":
		embed.Title = "General"
		embed.Description = "Prefix commands start with this. Slash commands and mentioning the bot always work.\n\n" +
			line("Prefix", "`"+v.guild.Prefix+"`", fmt.Sprintf("Up to %d characters, e.g. bt! or ?", store.MaxPrefixLength))
		rows = buttonRows(button("Change prefix", discordgo.SecondaryButton, v.id("edit", "prefix")), back)
	case "posting":
		embed.Title = "Posting"
		lines := []string{"How Boe Tea posts artwork from links.", line("Images per post", strconv.Itoa(v.guild.Posting.Limit), "Longer galleries are cut off after this many")}
		buttons := make([]discordgo.MessageComponent, 0, 8)
		for _, t := range v.spec.in("posting") {
			lines = append(lines, line(t.label, onOff(t.get(v.guild)), t.hint))
			buttons = append(buttons, v.toggleButton(t))
		}
		embed.Description = strings.Join(lines, "\n\n")
		buttons = append(buttons, button(fmt.Sprintf("Images per post: %d", v.guild.Posting.Limit), discordgo.SecondaryButton, v.id("edit", "limit")))
		rows = append(buttonRows(buttons...), buttonRows(back)...)
	case "sources":
		embed.Title = "Sources"
		embed.Description = "Links from turned-off sources are ignored."
		buttons := make([]discordgo.MessageComponent, 0, 4)
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
		rows = []discordgo.MessageComponent{v.modeSelect(), v.ttlSelect()}
		rows = append(rows, buttonRows(back)...)
	case "channels":
		embed.Title = fmt.Sprintf("Art channels · %d", len(v.guild.ArtChannels))
		embed.Description = channelsSummary(v.guild.ArtChannels, maxListedChannels)
		rows = []discordgo.MessageComponent{
			v.channelSelect("add", "Add channels or categories…"),
			v.channelSelect("remove", "Remove channels or categories…"),
		}
		rows = append(rows, buttonRows(back)...)
	default:
		v.renderHome(embed)
		rows = []discordgo.MessageComponent{v.navSelect()}
	}

	return &router.Response{Embeds: []*discordgo.MessageEmbed{embed}, Components: compact(rows)}
}

func (v *panelView) renderHome(embed *discordgo.MessageEmbed) {
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
	embed.Fields = []*discordgo.MessageEmbedField{
		{Name: "General", Value: "Prefix `" + g.Prefix + "`"},
		{Name: "Posting", Value: fmt.Sprintf(
			"Up to **%d** images per post · Tags **%s** · Reactions **%s**\nSkip first tweet image **%s** · Crossposting **%s**\nFooter quotes **%s** · NSFW quotes **%s**",
			p.Limit, onOff(p.Tags), onOff(p.Reactions), onOff(p.SkipFirstTweet), onOff(p.Crosspost), onOff(p.Quotes), allowed(p.NSFWQuotes),
		)},
		{Name: "Sources", Value: strings.Join(sources, " · ")},
		{Name: "Reposts", Value: reposts},
		{Name: fmt.Sprintf("Art channels · %d", len(g.ArtChannels)), Value: channelsSummary(g.ArtChannels, 15)},
	}
	embed.Footer = &discordgo.MessageEmbedFooter{Text: "Changing settings needs Manage Server"}
	if v.icon != "" {
		embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: v.icon}
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

func button(label string, style discordgo.ButtonStyle, id string) discordgo.MessageComponent {
	if id == "" {
		return nil
	}
	return discordgo.Button{Label: label, Style: style, CustomID: id}
}

func (v *panelView) toggleButton(t toggle) discordgo.MessageComponent {
	if t.get(v.guild) {
		return button(t.short+": On", discordgo.SuccessButton, v.id("toggle", t.name))
	}
	return button(t.short+": Off", discordgo.SecondaryButton, v.id("toggle", t.name))
}

func (v *panelView) navSelect() discordgo.MessageComponent {
	sections := v.spec.sections()
	options := make([]discordgo.SelectMenuOption, 0, len(sections))
	for _, s := range sections {
		options = append(options, discordgo.SelectMenuOption{Label: s.title, Value: s.key, Description: s.summary})
	}
	return v.stringSelect("nav", "Edit a section…", options)
}

func (v *panelView) modeSelect() discordgo.MessageComponent {
	options := make([]discordgo.SelectMenuOption, 0, len(repostModes))
	for _, m := range repostModes {
		options = append(options, discordgo.SelectMenuOption{
			Label: "Mode: " + m.label, Value: string(m.mode), Description: m.hint, Default: m.mode == v.guild.Repost.Mode,
		})
	}
	return v.stringSelect("mode", "Repost mode", options)
}

func (v *panelView) ttlSelect() discordgo.MessageComponent {
	options := make([]discordgo.SelectMenuOption, 0, len(ttlPresets))
	for _, p := range ttlPresets {
		options = append(options, discordgo.SelectMenuOption{
			Label: "Remember links for " + p.name, Value: p.value, Default: p.ttl == v.guild.Repost.TTL,
		})
	}
	return v.stringSelect("ttl", "Remember links for…", options)
}

func (v *panelView) stringSelect(action, placeholder string, options []discordgo.SelectMenuOption) discordgo.MessageComponent {
	id := v.id(action)
	if id == "" {
		return nil
	}

	return discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.SelectMenu{
		MenuType: discordgo.StringSelectMenu, CustomID: id, Placeholder: placeholder, Options: options,
	}}}
}

func (v *panelView) channelSelect(action, placeholder string) discordgo.MessageComponent {
	id := v.id(action)
	if id == "" {
		return nil
	}

	return discordgo.ActionsRow{Components: []discordgo.MessageComponent{discordgo.SelectMenu{
		MenuType:     discordgo.ChannelSelectMenu,
		CustomID:     id,
		Placeholder:  placeholder,
		MaxValues:    25,
		ChannelTypes: append(slices.Clone(artChannelTypes), discordgo.ChannelTypeGuildCategory),
	}}}
}

// buttonRows packs buttons into rows of Discord's maximum of five.
func buttonRows(buttons ...discordgo.MessageComponent) []discordgo.MessageComponent {
	var rows []discordgo.MessageComponent
	row := make([]discordgo.MessageComponent, 0, 5)
	for _, b := range buttons {
		if b == nil {
			continue
		}
		if len(row) == 5 {
			rows = append(rows, discordgo.ActionsRow{Components: row})
			row = make([]discordgo.MessageComponent, 0, 5)
		}
		row = append(row, b)
	}
	if len(row) > 0 {
		rows = append(rows, discordgo.ActionsRow{Components: row})
	}
	return rows
}

func compact(rows []discordgo.MessageComponent) []discordgo.MessageComponent {
	out := make([]discordgo.MessageComponent, 0, len(rows))
	for _, r := range rows {
		if r != nil {
			out = append(out, r)
		}
	}
	return out
}
