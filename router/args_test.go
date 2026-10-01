package router

import (
	"github.com/bwmarrin/discordgo"

	"github.com/onsi/ginkgo/v2"
	"github.com/onsi/gomega"
)

func testCtx() *Context {
	return &Context{Message: &discordgo.Message{GuildID: "1"}}
}

func testOK(_ *Context) error { return nil }

var _ = ginkgo.Describe("Tokenizing prefix arguments", func() {
	ginkgo.DescribeTable("splits on whitespace and honours quotes",
		func(in string, want []string) {
			got := tokenize(in)

			gomega.Expect(got).To(gomega.HaveLen(len(want)))

			for i := range got {
				gomega.Expect(got[i].text).To(gomega.Equal(want[i]))
				gomega.Expect(in[got[i].start:got[i].end]).NotTo(gomega.BeEmpty())
			}
		},
		ginkgo.Entry("empty input", ``, []string(nil)),
		ginkgo.Entry("plain words", `a b  c`, []string{"a", "b", "c"}),
		ginkgo.Entry("double quotes", `"hello world" x`, []string{"hello world", "x"}),
		ginkgo.Entry("single quotes", `'single quoted' y`, []string{"single quoted", "y"}),
		ginkgo.Entry("unterminated quote", `"unterminated x`, []string{`"unterminated`, "x"}),
		ginkgo.Entry("tabs and newlines", "tabs\tand\nnewlines", []string{"tabs", "and", "newlines"}),
	)
})

var _ = ginkgo.Describe("Parsing prefix options", func() {
	var cmd *Command

	ginkgo.BeforeEach(func() {
		cmd = &Command{
			Name:        "test",
			Description: "test",
			Options: []*Option{
				Integer("count", "count").Require().WithRange(1, 10),
				Boolean("flag", "flag"),
				Channel("channel", "channel"),
				String("query", "query").Greedy(),
			},
		}
	})

	ginkgo.It("maps positional arguments onto options", func() {
		opts, err := parsePrefixOptions(testCtx(), cmd, `5 yes <#123456789012345678> some long "quoted" query`)

		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(opts.Int("count")).To(gomega.Equal(int64(5)))
		gomega.Expect(opts.Bool("flag")).To(gomega.BeTrue())
		gomega.Expect(opts.ID("channel")).To(gomega.Equal("123456789012345678"))
		gomega.Expect(opts.String("query")).To(gomega.Equal(`some long "quoted" query`))
	})

	ginkgo.It("leaves optional trailing options absent", func() {
		opts, err := parsePrefixOptions(testCtx(), cmd, `3`)

		gomega.Expect(err).NotTo(gomega.HaveOccurred())
		gomega.Expect(opts.Has("flag")).To(gomega.BeFalse())
		gomega.Expect(opts.Has("query")).To(gomega.BeFalse())
		gomega.Expect(opts.BoolOr("flag", true)).To(gomega.BeTrue())
	})

	ginkgo.When("with invalid input", func() {
		var choiceCmd *Command

		ginkgo.BeforeEach(func() {
			choiceCmd = &Command{
				Name:        "test",
				Description: "test",
				Options: []*Option{
					Integer("count", "count").Require().WithRange(1, 10),
					String("mode", "mode").WithChoices(Choice{"Fast", "fast"}, Choice{"Slow", "slow"}),
				},
			}
		})

		ginkgo.It("reports missing required options", func() {
			_, err := parsePrefixOptions(testCtx(), choiceCmd, ``)

			gomega.Expect(err).To(gomega.BeAssignableToTypeOf(&OptionError{}))
			gomega.Expect(err).To(gomega.MatchError(ErrMissingOption))
		})

		ginkgo.It("rejects mistyped values", func() {
			_, err := parsePrefixOptions(testCtx(), choiceCmd, `abc`)

			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("rejects out-of-range values", func() {
			_, err := parsePrefixOptions(testCtx(), choiceCmd, `50`)

			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("rejects unknown choices", func() {
			_, err := parsePrefixOptions(testCtx(), choiceCmd, `5 medium`)

			gomega.Expect(err).To(gomega.HaveOccurred())
		})

		ginkgo.It("matches choices case-insensitively", func() {
			opts, err := parsePrefixOptions(testCtx(), choiceCmd, `5 FAST`)

			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			gomega.Expect(opts.String("mode")).To(gomega.Equal("fast"))
		})
	})
})

var _ = ginkgo.Describe("Validating commands", func() {
	ginkgo.DescribeTable("rejects malformed commands",
		func(cmd *Command) {
			gomega.Expect(cmd.validate(0)).To(gomega.HaveOccurred())
		},
		ginkgo.Entry("uppercase names", &Command{Name: "Upper", Description: "d", Handler: testOK}),
		ginkgo.Entry("missing handler", &Command{Name: "nohandler", Description: "d"}),
		ginkgo.Entry("missing description", &Command{Name: "nodesc", Handler: testOK}),
		ginkgo.Entry("group with handler", &Command{
			Name: "group", Description: "d", Handler: testOK,
			Subcommands: []*Command{{Name: "a", Description: "d", Handler: testOK}},
		}),
		ginkgo.Entry("required option after optional", &Command{
			Name: "opts", Description: "d", Handler: testOK,
			Options: []*Option{String("a", "d"), String("b", "d").Require()},
		}),
		ginkgo.Entry("greedy option not last", &Command{
			Name: "greedy", Description: "d", Handler: testOK,
			Options: []*Option{String("a", "d").Greedy(), String("b", "d")},
		}),
		ginkgo.Entry("too deeply nested", &Command{
			Name: "deep", Description: "d", Subcommands: []*Command{
				{Name: "l1", Description: "d", Subcommands: []*Command{
					{Name: "l2", Description: "d", Subcommands: []*Command{
						{Name: "l3", Description: "d", Handler: testOK},
					}},
				}},
			},
		}),
	)

	ginkgo.It("allows attachments after a greedy option", func() {
		cmd := &Command{Name: "sauce", Description: "d", Handler: testOK, Options: []*Option{
			String("url", "d").Require().Greedy(),
			Attachment("image", "d"),
		}}

		gomega.Expect(cmd.validate(0)).NotTo(gomega.HaveOccurred())
	})

	ginkgo.It("accepts nested groups", func() {
		good := &Command{Name: "set", Description: "d", Subcommands: []*Command{
			{Name: "prefix", Description: "d", Handler: testOK, Options: []*Option{String("value", "d").Require()}},
			{Name: "channel", Description: "d", Subcommands: []*Command{
				{Name: "add", Description: "d", Handler: testOK},
			}},
		}}

		gomega.Expect(good.validate(0)).NotTo(gomega.HaveOccurred())
		gomega.Expect(good.Subcommands[1].Subcommands[0].QualifiedName()).To(gomega.Equal("set channel add"))
		gomega.Expect(good.Subcommands[0].Usage("bt!")).To(gomega.Equal("bt!set prefix <value>"))

		ac := good.applicationCommand()

		gomega.Expect(ac.Options).To(gomega.HaveLen(2))
		gomega.Expect(ac.Options[1].Type).To(gomega.Equal(discordgo.ApplicationCommandOptionSubCommandGroup))
	})
})

var _ = ginkgo.Describe("Looking up commands", func() {
	ginkgo.It("resolves names and aliases", func() {
		r := New(Config{Prefixes: []string{"bt!"}})
		r.MustRegister(&Command{Name: "set", Description: "d", Aliases: []string{"config"}, Subcommands: []*Command{
			{Name: "prefix", Description: "d", Aliases: []string{"p"}, Handler: testOK},
		}})

		gomega.Expect(r.Lookup("config", "P")).NotTo(gomega.BeNil())
		gomega.Expect(r.Lookup("set", "nope")).To(gomega.BeNil())
	})

	ginkgo.It("rejects duplicate prefix keys", func() {
		r := New(Config{Prefixes: []string{"bt!"}})
		r.MustRegister(&Command{Name: "set", Description: "d", Aliases: []string{"config"}, Subcommands: []*Command{
			{Name: "prefix", Description: "d", Handler: testOK},
		}})

		gomega.Expect(r.Register(&Command{Name: "config", Description: "d", Handler: testOK})).To(gomega.HaveOccurred())
	})
})
