package router

import (
	"strings"

	"github.com/bwmarrin/discordgo"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func helpTestRouter() *Router {
	return New(Config{Prefixes: []string{"bt!"}})
}

func helpMessageCtx(r *Router) *Context {
	return &Context{
		Session: &discordgo.Session{},
		Router:  r,
		Message: &discordgo.Message{
			ID:        "m",
			ChannelID: "c",
			GuildID:   "g",
			Content:   "bt!help",
			Author:    &discordgo.User{ID: "u"},
		},
		Prefix:  "bt!",
		Options: newOptions(),
	}
}

func helpInteractionCtx(r *Router) *Context {
	return &Context{
		Session: &discordgo.Session{},
		Router:  r,
		Interaction: &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
			GuildID: "g",
			Member:  &discordgo.Member{User: &discordgo.User{ID: "u"}},
		}},
		Options: newOptions(),
	}
}

func helpTestCommand() *Command {
	return &Command{
		Name:        "sauce",
		Description: "d",
		Handler:     testOK,
		Options: []*Option{
			String("url", "d"),
		},
	}
}

func viewFor(ctx *Context) *helpView {
	help := HelpCommand(HelpConfig{})
	return newHelpView(ctx, HelpConfig{Name: help.Name}, help)
}

// helpRouter registers a small command set across categories.
func helpRouter(cfg HelpConfig) (*Router, *Command) {
	r := helpTestRouter()
	help := HelpCommand(cfg)
	r.MustRegister(
		help,
		&Command{
			Name: "set", Description: "Edits settings.", Category: "Settings", Aliases: []string{"cfg"}, Handler: testOK,
			Examples: []string{"set pixiv false"},
			Options: []*Option{
				String("setting", "Setting to change").WithChoices(
					Choice{Name: "a", Value: "a"}, Choice{Name: "b", Value: "b"}, Choice{Name: "c", Value: "c"},
					Choice{Name: "d", Value: "d"}, Choice{Name: "e", Value: "e"}, Choice{Name: "f", Value: "f"},
					Choice{Name: "g", Value: "g"}, Choice{Name: "h", Value: "h"},
				).Require(),
				String("value", "New value").Greedy(),
			},
		},
		&Command{Name: "sauce", Description: "Finds sauce.", Category: "Source", Handler: testOK},
		&Command{Name: "Find Sauce", Type: MessageContext, Description: "Finds sauce in a message.", Category: "Source", Handler: testOK},
		&Command{Name: "borgar", Description: "Borgar.", Category: "Memes", Handler: testOK},
		&Command{Name: "secret", Description: "Hidden.", Category: "Owner", Hidden: true, Handler: testOK},
	)
	return r, help
}

func newTestView(r *Router, help *Command, cfg HelpConfig, prefix bool) *helpView {
	cfg.Name = help.Name
	cfg.Uncategorized = "Other"
	cfg.Title = "Commands"
	return &helpView{
		r: r, s: &discordgo.Session{}, cfg: cfg, help: help,
		guildID: "g", channelID: "c", userID: "u", prefix: prefix,
	}
}

func componentClick(userID, customID string, values ...string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		Type:    discordgo.InteractionMessageComponent,
		GuildID: "g",
		Member:  &discordgo.Member{User: &discordgo.User{ID: userID}},
		Data:    discordgo.MessageComponentInteractionData{CustomID: customID, Values: values},
	}}
}

func selectMenu(resp *Response, row int) discordgo.SelectMenu {
	return resp.Components[row].(discordgo.ActionsRow).Components[0].(discordgo.SelectMenu)
}

func buttons(resp *Response, row int) []discordgo.Button {
	var out []discordgo.Button
	for _, c := range resp.Components[row].(discordgo.ActionsRow).Components {
		out = append(out, c.(discordgo.Button))
	}
	return out
}

var _ = ginkgo.Describe("Help syntax leads", func() {
	ginkgo.It("prefers the guild prefix for prefix invocations", func() {
		r := helpTestRouter()
		v := viewFor(helpMessageCtx(r))

		gomega.Expect(v.prefixLead()).To(gomega.Equal("bt!"))
		gomega.Expect(v.syntaxLead(helpTestCommand())).To(gomega.Equal("bt!"))
	})

	ginkgo.It("prefers slash syntax for interactions", func() {
		r := helpTestRouter()
		v := viewFor(helpInteractionCtx(r))

		gomega.Expect(v.syntaxLead(helpTestCommand())).To(gomega.Equal("/"))
	})

	ginkgo.It("falls back to slash without configured prefixes", func() {
		r := New(Config{})
		ctx := helpMessageCtx(r)
		ctx.Prefix = ""

		gomega.Expect(viewFor(ctx).prefixLead()).To(gomega.Equal("/"))
	})
})

var _ = ginkgo.Describe("Help usage lines", func() {
	ginkgo.It("lists prefix syntax first for prefix invocations", func() {
		r := helpTestRouter()

		gomega.Expect(viewFor(helpMessageCtx(r)).helpUsage(helpTestCommand())).To(gomega.Equal([]string{
			"bt!sauce [url]",
			"/sauce [url]",
		}))
	})

	ginkgo.It("lists slash syntax first for interactions", func() {
		r := helpTestRouter()

		gomega.Expect(viewFor(helpInteractionCtx(r)).helpUsage(helpTestCommand())).To(gomega.Equal([]string{
			"/sauce [url]",
			"bt!sauce [url]",
		}))
	})

	ginkgo.It("omits disabled forms", func() {
		r := helpTestRouter()
		cmd := helpTestCommand()
		cmd.DisableSlash = true

		gomega.Expect(viewFor(helpInteractionCtx(r)).helpUsage(cmd)).To(gomega.Equal([]string{
			"bt!sauce [url]",
		}))
	})
})

var _ = ginkgo.Describe("Help categories", func() {
	ginkgo.It("orders configured categories first, then the rest alphabetically", func() {
		r, help := helpRouter(HelpConfig{})
		v := newTestView(r, help, HelpConfig{CategoryOrder: []string{"Settings", "General"}}, false)

		var names []string
		for _, c := range v.categories() {
			names = append(names, c.name)
		}

		gomega.Expect(names).To(gomega.Equal([]string{"Settings", "General", "Memes", "Source"}))
	})

	ginkgo.It("lists context menus after chat commands and skips hidden commands", func() {
		r, help := helpRouter(HelpConfig{})
		v := newTestView(r, help, HelpConfig{}, false)

		for _, c := range v.categories() {
			gomega.Expect(c.name).NotTo(gomega.Equal("Owner"))
			if c.name == "Source" {
				gomega.Expect(c.cmds).To(gomega.HaveLen(2))
				gomega.Expect(c.cmds[0].Name).To(gomega.Equal("sauce"))
				gomega.Expect(c.cmds[1].Name).To(gomega.Equal("Find Sauce"))
			}
		}
	})
})

var _ = ginkgo.Describe("Help overview", func() {
	ginkgo.It("shows a field and a menu option per category", func() {
		r, help := helpRouter(HelpConfig{})
		resp := newTestView(r, help, HelpConfig{}, false).home()

		embed := resp.Embeds[0]
		gomega.Expect(embed.Fields).To(gomega.HaveLen(4))
		gomega.Expect(embed.Fields[0].Inline).To(gomega.BeTrue())
		gomega.Expect(embed.Description).To(gomega.ContainSubstring("`/help <command>`"))
		gomega.Expect(embed.Description).To(gomega.ContainSubstring("`bt!`"))

		menu := selectMenu(resp, 0)
		gomega.Expect(menu.CustomID).To(gomega.Equal("rt:help:u:s:cat"))
		gomega.Expect(menu.Options).To(gomega.HaveLen(4))

		source := menu.Options[3]
		gomega.Expect(source.Value).To(gomega.Equal("Source"))
		gomega.Expect(source.Description).To(gomega.Equal("sauce, Find Sauce"))
	})
})

var _ = ginkgo.Describe("Help category view", func() {
	ginkgo.It("lists commands with descriptions and a command menu", func() {
		r, help := helpRouter(HelpConfig{})
		resp := newTestView(r, help, HelpConfig{}, false).category("Source")

		embed := resp.Embeds[0]
		gomega.Expect(embed.Title).To(gomega.Equal("Source"))
		gomega.Expect(embed.Description).To(gomega.Equal(
			"`/sauce` — Finds sauce.\n**Find Sauce** — Finds sauce in a message. _(right-click a message, Apps)_"))

		menu := selectMenu(resp, 0)
		gomega.Expect(menu.CustomID).To(gomega.Equal("rt:help:u:s:cmd"))
		gomega.Expect(menu.Options[1].Value).To(gomega.Equal("1:Find Sauce"))

		gomega.Expect(buttons(resp, 1)[0].CustomID).To(gomega.Equal("rt:help:u:s:home"))
	})

	ginkgo.It("uses clickable mentions once commands are synced", func() {
		r, help := helpRouter(HelpConfig{})
		r.commandIDs[slashKey{ChatInput, "sauce"}] = "42"

		resp := newTestView(r, help, HelpConfig{}, false).category("Source")
		gomega.Expect(resp.Embeds[0].Description).To(gomega.HavePrefix("</sauce:42> — Finds sauce."))
	})

	ginkgo.It("uses prefix syntax for prefix invocations", func() {
		r, help := helpRouter(HelpConfig{})
		r.commandIDs[slashKey{ChatInput, "sauce"}] = "42"

		resp := newTestView(r, help, HelpConfig{}, true).category("Source")
		gomega.Expect(resp.Embeds[0].Description).To(gomega.HavePrefix("`bt!sauce` — Finds sauce."))
	})
})

var _ = ginkgo.Describe("Help detail view", func() {
	ginkgo.It("shows usage, other forms, options, choices and navigation", func() {
		r, help := helpRouter(HelpConfig{})
		v := newTestView(r, help, HelpConfig{}, false)
		resp := v.detail(r.Lookup("set"))

		embed := resp.Embeds[0]
		gomega.Expect(embed.Author.Name).To(gomega.Equal("Settings"))
		gomega.Expect(embed.Title).To(gomega.Equal("/set"))
		gomega.Expect(embed.Description).To(gomega.Equal(
			"Edits settings.\n```\n/set <setting> [value...]\n```\n-# Also `bt!set`, `bt!cfg`"))

		gomega.Expect(embed.Fields[0].Name).To(gomega.Equal("Options"))
		gomega.Expect(embed.Fields[0].Value).To(gomega.Equal(
			"`setting` _required_ — Setting to change\n-# a · b · c · d · e · f · +2 more\n" +
				"`value` _optional, takes the rest_ — New value"))

		gomega.Expect(embed.Fields[1].Name).To(gomega.Equal("Example"))
		gomega.Expect(embed.Fields[1].Value).To(gomega.Equal("`/set pixiv false`"))

		row := buttons(resp, 0)
		gomega.Expect(row).To(gomega.HaveLen(2))
		gomega.Expect(row[0].Label).To(gomega.Equal("Back to Settings"))
		gomega.Expect(row[0].CustomID).To(gomega.Equal("rt:help:u:s:cat:Settings"))
	})

	ginkgo.It("leads with prefix syntax over prefix", func() {
		r, help := helpRouter(HelpConfig{})
		embed := newTestView(r, help, HelpConfig{}, true).detail(r.Lookup("set")).Embeds[0]

		gomega.Expect(embed.Title).To(gomega.Equal("bt!set"))
		gomega.Expect(embed.Description).To(gomega.HaveSuffix("\n-# Also `/set`, `bt!cfg`"))
	})

	ginkgo.It("renders examples as mentions once synced", func() {
		r, help := helpRouter(HelpConfig{})
		r.commandIDs[slashKey{ChatInput, "set"}] = "7"

		embed := newTestView(r, help, HelpConfig{}, false).detail(r.Lookup("set")).Embeds[0]
		gomega.Expect(embed.Fields[1].Value).To(gomega.Equal("</set:7> `pixiv false`"))
	})

	ginkgo.It("finds context menu commands by name", func() {
		r, help := helpRouter(HelpConfig{})
		v := newTestView(r, help, HelpConfig{}, false)

		cmd := v.lookup("find sauce", "")
		gomega.Expect(cmd).NotTo(gomega.BeNil())

		embed := v.detail(cmd).Embeds[0]
		gomega.Expect(embed.Title).To(gomega.Equal("Find Sauce (message menu)"))
		gomega.Expect(embed.Description).To(gomega.ContainSubstring("Right-click a message → Apps → Find Sauce"))
	})

	ginkgo.It("does not find hidden commands", func() {
		r, help := helpRouter(HelpConfig{})
		gomega.Expect(newTestView(r, help, HelpConfig{}, false).lookup("secret", "")).To(gomega.BeNil())
	})
})

var _ = ginkgo.Describe("Help menu clicks", func() {
	respond := func(r *Router, help *Command, i *discordgo.InteractionCreate) *discordgo.InteractionResponse {
		args := strings.TrimPrefix(i.MessageComponentData().CustomID, componentPrefix+help.Name+":")
		return helpComponentResponse(r, &discordgo.Session{}, HelpConfig{Name: "help", Title: "Commands", Uncategorized: "Other"}, help, i, args)
	}

	ginkgo.It("opens the picked category in place", func() {
		r, help := helpRouter(HelpConfig{})
		resp := respond(r, help, componentClick("u", "rt:help:u:s:cat", "Memes"))

		gomega.Expect(resp.Type).To(gomega.Equal(discordgo.InteractionResponseUpdateMessage))
		gomega.Expect(resp.Data.Embeds[0].Title).To(gomega.Equal("Memes"))
	})

	ginkgo.It("opens a picked context menu command", func() {
		r, help := helpRouter(HelpConfig{})
		resp := respond(r, help, componentClick("u", "rt:help:u:s:cmd", "1:Find Sauce"))

		gomega.Expect(resp.Data.Embeds[0].Title).To(gomega.Equal("Find Sauce (message menu)"))
	})

	ginkgo.It("goes back to a category from a button", func() {
		r, help := helpRouter(HelpConfig{})
		resp := respond(r, help, componentClick("u", "rt:help:u:p:cat:Settings"))

		gomega.Expect(resp.Data.Embeds[0].Description).To(gomega.HavePrefix("`bt!set`"))
	})

	ginkgo.It("falls back to the overview for unknown targets", func() {
		r, help := helpRouter(HelpConfig{})
		resp := respond(r, help, componentClick("u", "rt:help:u:s:cmd", "0:nope"))

		gomega.Expect(resp.Data.Embeds[0].Title).To(gomega.Equal("Commands"))
	})

	ginkgo.It("answers other users privately without touching the menu", func() {
		r, help := helpRouter(HelpConfig{})
		resp := respond(r, help, componentClick("someone-else", "rt:help:u:s:home"))

		gomega.Expect(resp.Type).To(gomega.Equal(discordgo.InteractionResponseChannelMessageWithSource))
		gomega.Expect(resp.Data.Flags).To(gomega.Equal(discordgo.MessageFlagsEphemeral))
	})
})
