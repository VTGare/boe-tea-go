package widget

import (
	"context"
	"errors"
	"time"

	gt "github.com/VTGare/gumi/v2/gumitest"
	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var errTestFailure = errors.New("test failure")

const (
	author   snowflake.ID = 7
	stranger snowflake.ID = 8
)

func testPages() []*discord.Embed {
	return []*discord.Embed{{Title: "one"}, {Title: "two"}, {Title: "three"}}
}

func click(c *bot.Client, userID snowflake.ID, customID string) *events.ComponentInteractionCreate {
	e := gt.Button(userID, customID).Event(c)

	return &events.ComponentInteractionCreate{
		GenericEvent:         e.GenericEvent,
		ComponentInteraction: e.Interaction.(discord.ComponentInteraction),
		Respond:              e.Respond,
	}
}

func embedTitle(response map[string]any) string {
	embeds, _ := response["embeds"].([]any)
	if len(embeds) == 0 {
		return ""
	}

	title, _ := embeds[0].(map[string]any)["title"].(string)

	return title
}

func ephemeral(response map[string]any) bool {
	flags, _ := response["flags"].(float64)

	return discord.MessageFlags(flags).Has(discord.MessageFlagEphemeral)
}

func actionID(w *Widget, action Action) string {
	return customID(w.id, action)
}

var _ = Describe("Custom IDs", func() {
	It("round-trips widget IDs and actions", func() {
		for a := ActionFirstPage; a <= ActionStop; a++ {
			wid, got, ok := parseCustomID(customID("abc123", a))

			Expect(ok).To(BeTrue())
			Expect(wid).To(Equal("abc123"))
			Expect(got).To(Equal(a))
		}
	})

	It("rejects foreign IDs", func() {
		for _, id := range []string{"", "other:page:abc:next", "boe:page:abc", "boe:page:abc:bogus"} {
			_, _, ok := parseCustomID(id)

			Expect(ok).To(BeFalse(), "expected %q to be rejected", id)
		}
	})
})

var _ = Describe("Controls", func() {
	It("renders nothing for a single page", func() {
		w := New(author, testPages()[:1])

		Expect(w.Controls()).To(BeEmpty())
	})

	It("disables backward controls on the first page", func() {
		w := New(author, testPages())
		row, ok := w.Controls()[0].(discord.ActionRowComponent)

		Expect(ok).To(BeTrue())
		Expect(row.Components).To(HaveLen(5))

		buttons := make([]discord.ButtonComponent, 0, 5)
		for _, c := range row.Components {
			button, ok := c.(discord.ButtonComponent)

			Expect(ok).To(BeTrue())
			buttons = append(buttons, button)
		}

		Expect(buttons[0].Disabled).To(BeTrue())
		Expect(buttons[1].Disabled).To(BeTrue())
		Expect(buttons[2].Disabled).To(BeFalse())
		Expect(buttons[3].Disabled).To(BeFalse())
		Expect(buttons[4].Disabled).To(BeFalse())
	})
})

var _ = Describe("Widget", func() {
	var (
		c    *bot.Client
		rec  *gt.Recorder
		disp *Dispatcher
	)

	BeforeEach(func() {
		c, rec = gt.NewClient()
		disp = NewDispatcher()
	})

	serveWidget := func(w *Widget) chan error {
		w.Timeout = 5 * time.Second

		done := make(chan error, 1)
		go func() {
			done <- w.Serve(context.Background(), disp)
		}()

		Eventually(func() bool {
			_, ok := disp.lookup(w.id)
			return ok
		}).Should(BeTrue())

		return done
	}

	It("flips pages on the author's buttons", func() {
		w := New(author, testPages())
		done := serveWidget(w)

		disp.Handle(click(c, author, actionID(w, ActionNextPage)))

		Eventually(rec.Responses).Should(HaveLen(1))
		Expect(rec.ResponseTypes()[0]).To(Equal(discord.InteractionResponseTypeUpdateMessage))
		Expect(rec.Responses()[0]["embeds"]).To(HaveLen(1))
		Expect(embedTitle(rec.Responses()[0])).To(Equal("two"))

		disp.Handle(click(c, author, actionID(w, ActionLastPage)))

		Eventually(rec.Responses).Should(HaveLen(2))
		Expect(embedTitle(rec.Responses()[1])).To(Equal("three"))

		disp.Handle(click(c, author, actionID(w, ActionFirstPage)))

		Eventually(rec.Responses).Should(HaveLen(3))
		Expect(embedTitle(rec.Responses()[2])).To(Equal("one"))

		disp.Handle(click(c, author, actionID(w, ActionStop)))

		Eventually(done).Should(Receive(Succeed()))
	})

	It("strips buttons on stop", func() {
		w := New(author, testPages())
		done := serveWidget(w)

		disp.Handle(click(c, author, actionID(w, ActionStop)))

		Eventually(done).Should(Receive(Succeed()))
		Eventually(rec.Responses).Should(HaveLen(1))
		Expect(rec.ResponseTypes()[0]).To(Equal(discord.InteractionResponseTypeUpdateMessage))
		Expect(rec.Responses()[0]).To(HaveKeyWithValue("components", BeEmpty()))
	})

	It("rejects other users without flipping", func() {
		w := New(author, testPages())
		done := serveWidget(w)

		disp.Handle(click(c, stranger, actionID(w, ActionNextPage)))

		Eventually(rec.Responses).Should(HaveLen(1))
		Expect(rec.ResponseTypes()[0]).To(Equal(discord.InteractionResponseTypeCreateMessage))
		Expect(ephemeral(rec.Responses()[0])).To(BeTrue())

		Consistently(done, 50*time.Millisecond).ShouldNot(Receive())

		disp.Handle(click(c, author, actionID(w, ActionStop)))
		Eventually(done).Should(Receive(Succeed()))
	})

	It("expires menus that are gone", func() {
		w := New(author, testPages())

		disp.Handle(click(c, author, actionID(w, ActionNextPage)))

		Eventually(rec.Responses).Should(HaveLen(1))
		Expect(rec.Responses()[0]["content"]).To(Equal("This menu has expired."))
	})

	It("ignores other components", func() {
		w := New(author, testPages())
		done := serveWidget(w)

		disp.Handle(click(c, author, "other:thing"))
		disp.Handle(click(c, author, "rt:settings:x"))

		Consistently(rec.Responses, 50*time.Millisecond).Should(BeEmpty())

		disp.Handle(click(c, author, actionID(w, ActionStop)))
		Eventually(done).Should(Receive(Succeed()))
	})

	It("runs the callback before each page edit", func() {
		var actions []Action

		w := New(author, testPages())
		w.WithCallback(func(action Action, _ int) error {
			actions = append(actions, action)

			return nil
		})
		done := serveWidget(w)

		disp.Handle(click(c, author, actionID(w, ActionNextPage)))

		Eventually(rec.Responses).Should(HaveLen(1))
		Expect(actions).To(Equal([]Action{ActionNextPage}))

		disp.Handle(click(c, author, actionID(w, ActionStop)))
		Eventually(done).Should(Receive(Succeed()))
	})

	It("ends the run on callback failure", func() {
		w := New(author, testPages())
		w.WithCallback(func(Action, int) error {
			return errTestFailure
		})
		done := serveWidget(w)

		disp.Handle(click(c, author, actionID(w, ActionNextPage)))

		Eventually(done).Should(Receive(MatchError(errTestFailure)))
		Eventually(rec.Responses).Should(HaveLen(1))
		Expect(ephemeral(rec.Responses()[0])).To(BeTrue())
	})

	It("returns immediately for a single page", func() {
		w := New(author, testPages()[:1])

		Expect(w.Serve(context.Background(), disp)).To(Succeed())
	})

	It("ends on timeout", func() {
		w := New(author, testPages())
		w.Timeout = 20 * time.Millisecond

		Expect(w.Serve(context.Background(), disp)).To(Succeed())

		disp.Handle(click(c, author, actionID(w, ActionNextPage)))

		Eventually(rec.Responses).Should(HaveLen(1))
		Expect(rec.Responses()[0]["content"]).To(Equal("This menu has expired."))
	})
})
