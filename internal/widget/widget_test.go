package widget

import (
	"context"
	"time"

	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/bwmarrin/discordgo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func testPages() []*discordgo.MessageEmbed {
	return []*discordgo.MessageEmbed{{Title: "one"}, {Title: "two"}, {Title: "three"}}
}

func runWidget(w *Widget, channel string) chan error {
	done := make(chan error, 1)

	go func() {
		done <- w.Start(context.Background(), channel)
	}()

	return done
}

var _ = Describe("Widget", func() {
	var (
		fake   *sender.FakeSender
		events *FakeSource
	)

	BeforeEach(func() {
		fake = sender.NewFake()
		events = NewFakeSource()
		events.BotID = "bot"
	})

	newWidget := func(pages []*discordgo.MessageEmbed) *Widget {
		w := New(fake, events, "g", "author", pages)
		w.Timeout = 200 * time.Millisecond

		return w
	}

	It("sends single pages without controls or a loop", func() {
		w := newWidget(testPages()[:1])

		Expect(w.Start(context.Background(), "c")).To(Succeed())
		Expect(fake.EmbedsSent()).To(HaveLen(1))
		Expect(fake.ReactionsAdded()).To(BeEmpty())
	})

	It("surfaces send failures", func() {
		fake.Skip = true
		w := newWidget(testPages())

		Expect(w.Start(context.Background(), "c")).To(MatchError(sender.ErrSkipped))
	})

	It("advances pages on the author's controls", func() {
		w := newWidget(testPages())
		done := runWidget(w, "c")

		Eventually(fake.EmbedsSent).Should(HaveLen(1))
		msgID := fake.EmbedsSent()[0].Sent.ID

		events.Emit(msgID, "author", "▶")
		Eventually(fake.EmbedsEdited).Should(HaveLen(1))
		Expect(fake.EmbedsEdited()[0].Embed.Title).To(Equal("two"))

		events.Emit(msgID, "author", "⏭")
		Eventually(fake.EmbedsEdited).Should(HaveLen(2))
		Expect(fake.EmbedsEdited()[1].Embed.Title).To(Equal("three"))

		events.Emit(msgID, "author", "◀")
		Eventually(fake.EmbedsEdited).Should(HaveLen(3))

		events.Close()
		Expect(<-done).To(Succeed())

		Expect(fake.ReactionsRemoved()).To(HaveLen(3))
	})

	It("ignores other users and its own reactions", func() {
		w := newWidget(testPages())
		done := runWidget(w, "c")

		Eventually(fake.EmbedsSent).Should(HaveLen(1))
		msgID := fake.EmbedsSent()[0].Sent.ID

		events.Emit(msgID, "stranger", "▶")
		events.Emit(msgID, "bot", "▶")

		Consistently(fake.EmbedsEdited, 50*time.Millisecond).Should(BeEmpty())

		events.Close()
		Expect(<-done).To(Succeed())
	})

	It("clears reactions and stops on the stop control", func() {
		w := newWidget(testPages())
		done := runWidget(w, "c")

		Eventually(fake.EmbedsSent).Should(HaveLen(1))
		msgID := fake.EmbedsSent()[0].Sent.ID

		events.Emit(msgID, "author", "⏹")

		Expect(<-done).To(Succeed())
		Expect(fake.ReactionsCleared()).To(HaveLen(1))
		Expect(fake.ReactionsCleared()[0].MessageID).To(Equal(msgID))
	})

	It("runs the callback before each page edit", func() {
		var actions []Action

		w := newWidget(testPages())
		w.WithCallback(func(action Action, _ int) error {
			actions = append(actions, action)

			return nil
		})

		done := runWidget(w, "c")

		Eventually(fake.EmbedsSent).Should(HaveLen(1))
		msgID := fake.EmbedsSent()[0].Sent.ID

		events.Emit(msgID, "author", "▶")
		Eventually(fake.EmbedsEdited).Should(HaveLen(1))

		events.Close()
		Expect(<-done).To(Succeed())
		Expect(actions).To(Equal([]Action{ActionNextPage}))
	})

	It("ends on timeout", func() {
		w := newWidget(testPages())
		w.Timeout = 20 * time.Millisecond

		Expect(w.Start(context.Background(), "c")).To(Succeed())
	})
})
