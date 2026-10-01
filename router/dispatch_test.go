package router

import (
	"github.com/bwmarrin/discordgo"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

var _ = ginkgo.Describe("Prefix dispatch", func() {
	var (
		r    *Router
		ran  string
		opts *Options
		err  error
	)

	record := func(name string) Handler {
		return func(ctx *Context) error {
			ran, opts = name, ctx.Options
			return nil
		}
	}

	send := func(content string) {
		r.dispatchMessage(&discordgo.Session{}, &discordgo.MessageCreate{Message: &discordgo.Message{Content: content}})
	}

	ginkgo.BeforeEach(func() {
		ran, opts, err = "", nil, nil
		r = New(Config{
			Prefixes:     []string{"bt!"},
			ErrorHandler: func(_ *Context, e error) { err = e },
		})
	})

	ginkgo.Describe("groups with a default subcommand", func() {
		ginkgo.BeforeEach(func() {
			r.MustRegister(&Command{
				Name: "bookmarks", Description: "d", Aliases: []string{"favs"}, Default: "list",
				Subcommands: []*Command{
					{Name: "list", Description: "d", Handler: record("list"), Options: []*Option{
						String("sort", "d").WithChoices(Choice{"time", "time"}, Choice{"popularity", "popularity"}),
					}},
					{Name: "remove", Description: "d", Handler: record("remove"), Options: []*Option{
						String("query", "d").Require(),
					}},
				},
			})
		})

		ginkgo.It("runs the default without arguments", func() {
			send("bt!favs")

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(ran).To(gomega.Equal("list"))
		})

		ginkgo.It("passes a non-subcommand word to the default", func() {
			send("bt!bookmarks popularity")

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(ran).To(gomega.Equal("list"))
			gomega.Expect(opts.String("sort")).To(gomega.Equal("popularity"))
		})

		ginkgo.It("still picks named subcommands", func() {
			send("bt!bookmarks remove 69")

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(ran).To(gomega.Equal("remove"))
			gomega.Expect(opts.String("query")).To(gomega.Equal("69"))
		})
	})

	ginkgo.It("asks for a subcommand when a group has no default", func() {
		r.MustRegister(&Command{Name: "groups", Description: "d", Subcommands: []*Command{
			{Name: "list", Description: "d", Handler: record("list")},
		}})

		send("bt!groups")

		gomega.Expect(ran).To(gomega.BeEmpty())
		gomega.Expect(err).To(gomega.BeAssignableToTypeOf(&SubcommandError{}))
	})

	ginkgo.It("skips slash-only options", func() {
		r.MustRegister(&Command{Name: "share", Description: "d", Handler: record("share"), Options: []*Option{
			String("url", "d").Require(),
			String("images", "d").Greedy(),
			String("mode", "d").SlashOnly().WithChoices(Choice{"include", "include"}, Choice{"exclude", "exclude"}),
		}})

		send("bt!share https://x.com/a 1-3 5")

		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(opts.String("images")).To(gomega.Equal("1-3 5"))
		gomega.Expect(opts.Has("mode")).To(gomega.BeFalse())
	})
})

var _ = ginkgo.Describe("Validating slash-only options and defaults", func() {
	cmd := func(opts ...*Option) *Command {
		return &Command{Name: "share", Description: "d", Handler: testOK, Options: opts}
	}

	ginkgo.It("may follow a greedy option and are left out of prefix usage", func() {
		c := cmd(String("url", "d").Require().Greedy(), Boolean("private", "d").SlashOnly())

		gomega.Expect(c.validate(0)).NotTo(gomega.HaveOccurred())
		gomega.Expect(c.Usage("bt!")).To(gomega.Equal("bt!share <url...>"))
		gomega.Expect(c.Usage("/")).To(gomega.Equal("/share <url...> [private]"))
	})

	ginkgo.It("cannot be required", func() {
		gomega.Expect(cmd(String("mode", "d").Require().SlashOnly()).validate(0)).To(gomega.HaveOccurred())
	})

	ginkgo.It("rejects a default that is not a subcommand", func() {
		c := &Command{Name: "g", Description: "d", Default: "nope", Subcommands: []*Command{
			{Name: "list", Description: "d", Handler: testOK},
		}}

		gomega.Expect(c.validate(0)).To(gomega.HaveOccurred())
	})
})
