package widget

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var errTestFailure = errors.New("test failure")

func testPages() []*discordgo.MessageEmbed {
	return []*discordgo.MessageEmbed{{Title: "one"}, {Title: "two"}, {Title: "three"}}
}

type recordedResponse struct {
	Type discordgo.InteractionResponseType `json:"type"`
	Data responseData                      `json:"data"`
}

type responseData struct {
	Embeds     []*discordgo.MessageEmbed `json:"embeds"`
	Components []json.RawMessage         `json:"components"`
	Content    string                    `json:"content"`
	Flags      discordgo.MessageFlags    `json:"flags"`
}

// testAPI reroutes Discord REST calls at the test server and records
// every interaction response.
type testAPI struct {
	mu      sync.Mutex
	records []recordedResponse
}

func (a *testAPI) handler(w http.ResponseWriter, r *http.Request) {
	var resp recordedResponse
	Expect(json.NewDecoder(r.Body).Decode(&resp)).To(Succeed())

	a.mu.Lock()
	defer a.mu.Unlock()

	a.records = append(a.records, resp)
	w.WriteHeader(http.StatusOK)
}

func (a *testAPI) responses() []recordedResponse {
	a.mu.Lock()
	defer a.mu.Unlock()

	return append([]recordedResponse(nil), a.records...)
}

func testSession(api *testAPI) *discordgo.Session {
	ts := httptest.NewServer(http.HandlerFunc(api.handler))
	DeferCleanup(ts.Close)

	oldEndpoint := discordgo.EndpointAPI
	discordgo.EndpointAPI = ts.URL + "/api/v9/"
	DeferCleanup(func() { discordgo.EndpointAPI = oldEndpoint })

	s, err := discordgo.New("Bot test")
	Expect(err).NotTo(HaveOccurred())
	s.Client = ts.Client()

	return s
}

func clickInteraction(userID, customID string) *discordgo.InteractionCreate {
	return &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
		ID:    "interaction-1",
		Token: "token-1",
		Type:  discordgo.InteractionMessageComponent,
		Member: &discordgo.Member{
			GuildID: "g",
			User:    &discordgo.User{ID: userID},
		},
		Message: &discordgo.Message{ID: "m", ChannelID: "c"},
		Data: discordgo.MessageComponentInteractionData{
			CustomID:      customID,
			ComponentType: discordgo.ButtonComponent,
		},
	}}
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
		w := New("author", testPages()[:1])

		Expect(w.Controls()).To(BeEmpty())
	})

	It("disables backward controls on the first page", func() {
		w := New("author", testPages())
		row, ok := w.Controls()[0].(discordgo.ActionsRow)

		Expect(ok).To(BeTrue())
		Expect(row.Components).To(HaveLen(5))

		buttons := make([]discordgo.Button, 0, 5)
		for _, c := range row.Components {
			button, ok := c.(discordgo.Button)

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
		api  *testAPI
		s    *discordgo.Session
		disp *Dispatcher
	)

	BeforeEach(func() {
		api = &testAPI{}
		s = testSession(api)
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
		w := New("author", testPages())
		done := serveWidget(w)

		disp.Handle(s, clickInteraction("author", actionID(w, ActionNextPage)))

		Eventually(api.responses).Should(HaveLen(1))
		Expect(api.responses()[0].Type).To(Equal(discordgo.InteractionResponseUpdateMessage))
		Expect(api.responses()[0].Data.Embeds).To(HaveLen(1))
		Expect(api.responses()[0].Data.Embeds[0].Title).To(Equal("two"))

		disp.Handle(s, clickInteraction("author", actionID(w, ActionLastPage)))

		Eventually(api.responses).Should(HaveLen(2))
		Expect(api.responses()[1].Data.Embeds[0].Title).To(Equal("three"))

		disp.Handle(s, clickInteraction("author", actionID(w, ActionFirstPage)))

		Eventually(api.responses).Should(HaveLen(3))
		Expect(api.responses()[2].Data.Embeds[0].Title).To(Equal("one"))

		disp.Handle(s, clickInteraction("author", actionID(w, ActionStop)))

		Eventually(done).Should(Receive(Succeed()))
	})

	It("strips buttons on stop", func() {
		w := New("author", testPages())
		done := serveWidget(w)

		disp.Handle(s, clickInteraction("author", actionID(w, ActionStop)))

		Eventually(done).Should(Receive(Succeed()))
		Eventually(api.responses).Should(HaveLen(1))
		Expect(api.responses()[0].Type).To(Equal(discordgo.InteractionResponseUpdateMessage))
		Expect(api.responses()[0].Data.Components).To(BeEmpty())
	})

	It("rejects other users without flipping", func() {
		w := New("author", testPages())
		done := serveWidget(w)

		disp.Handle(s, clickInteraction("stranger", actionID(w, ActionNextPage)))

		Eventually(api.responses).Should(HaveLen(1))
		Expect(api.responses()[0].Type).To(Equal(discordgo.InteractionResponseChannelMessageWithSource))
		Expect(api.responses()[0].Data.Flags & discordgo.MessageFlagsEphemeral).NotTo(BeZero())

		Consistently(done, 50*time.Millisecond).ShouldNot(Receive())

		disp.Handle(s, clickInteraction("author", actionID(w, ActionStop)))
		Eventually(done).Should(Receive(Succeed()))
	})

	It("expires menus that are gone", func() {
		w := New("author", testPages())

		disp.Handle(s, clickInteraction("author", actionID(w, ActionNextPage)))

		Eventually(api.responses).Should(HaveLen(1))
		Expect(api.responses()[0].Data.Content).To(Equal("This menu has expired."))
	})

	It("ignores other components and events", func() {
		w := New("author", testPages())
		done := serveWidget(w)

		foreign := clickInteraction("author", "other:thing")
		disp.Handle(s, foreign)

		nonComponent := clickInteraction("author", actionID(w, ActionNextPage))
		nonComponent.Type = discordgo.InteractionApplicationCommand
		disp.Handle(s, nonComponent)

		disp.Handle(s, nil)

		Consistently(api.responses, 50*time.Millisecond).Should(BeEmpty())

		disp.Handle(s, clickInteraction("author", actionID(w, ActionStop)))
		Eventually(done).Should(Receive(Succeed()))
	})

	It("runs the callback before each page edit", func() {
		var actions []Action

		w := New("author", testPages())
		w.WithCallback(func(action Action, _ int) error {
			actions = append(actions, action)

			return nil
		})
		done := serveWidget(w)

		disp.Handle(s, clickInteraction("author", actionID(w, ActionNextPage)))

		Eventually(api.responses).Should(HaveLen(1))
		Expect(actions).To(Equal([]Action{ActionNextPage}))

		disp.Handle(s, clickInteraction("author", actionID(w, ActionStop)))
		Eventually(done).Should(Receive(Succeed()))
	})

	It("ends the run on callback failure", func() {
		w := New("author", testPages())
		w.WithCallback(func(Action, int) error {
			return errTestFailure
		})
		done := serveWidget(w)

		disp.Handle(s, clickInteraction("author", actionID(w, ActionNextPage)))

		Eventually(done).Should(Receive(MatchError(errTestFailure)))
		Eventually(api.responses).Should(HaveLen(1))
		Expect(api.responses()[0].Data.Flags & discordgo.MessageFlagsEphemeral).NotTo(BeZero())
	})

	It("returns immediately for a single page", func() {
		w := New("author", testPages()[:1])

		Expect(w.Serve(context.Background(), disp)).To(Succeed())
	})

	It("ends on timeout", func() {
		w := New("author", testPages())
		w.Timeout = 20 * time.Millisecond

		Expect(w.Serve(context.Background(), disp)).To(Succeed())

		disp.Handle(s, clickInteraction("author", actionID(w, ActionNextPage)))

		Eventually(api.responses).Should(HaveLen(1))
		Expect(api.responses()[0].Data.Content).To(Equal("This menu has expired."))
	})
})
