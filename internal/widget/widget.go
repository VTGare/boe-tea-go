package widget

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// DefaultTimeout is how long a widget accepts clicks.
const DefaultTimeout = 2 * time.Minute

// customPrefix marks widget button IDs, so the Dispatcher can ignore
// other components.
const customPrefix = "boe:page"

// Action is a pagination button.
type Action int

const (
	ActionFirstPage Action = iota
	ActionPreviousPage
	ActionNextPage
	ActionLastPage
	ActionStop
)

func (a Action) String() string {
	switch a {
	case ActionFirstPage:
		return "first"
	case ActionPreviousPage:
		return "prev"
	case ActionNextPage:
		return "next"
	case ActionLastPage:
		return "last"
	case ActionStop:
		return "stop"
	}

	return "unknown"
}

// Widget is a paginated message. Clicks are handled one at a time. Set
// the callback before Serve, and don't call widget methods from inside it.
type Widget struct {
	author   string
	Pages    []*discordgo.MessageEmbed
	current  int
	callback func(Action, int) error
	Timeout  time.Duration

	mu   sync.Mutex
	id   string
	done chan error
	once sync.Once
}

func New(author string, pages []*discordgo.MessageEmbed) *Widget {
	return &Widget{
		author:  author,
		Pages:   pages,
		Timeout: DefaultTimeout,
		id:      newWidgetID(),
		done:    make(chan error, 1),
	}
}

// WithCallback runs fn after every handled control, before the page edit.
func (w *Widget) WithCallback(fn func(Action, int) error) {
	w.callback = fn
}

// Controls renders the button row for the current page, or nil when there
// is nothing to flip through.
func (w *Widget) Controls() []discordgo.MessageComponent {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.controls()
}

func (w *Widget) controls() []discordgo.MessageComponent {
	if len(w.Pages) <= 1 {
		return nil
	}

	last := len(w.Pages) - 1
	atFirst := w.current == 0
	atLast := w.current == last

	return []discordgo.MessageComponent{
		discordgo.ActionsRow{
			Components: []discordgo.MessageComponent{
				w.button(ActionFirstPage, "First", discordgo.SecondaryButton, atFirst),
				w.button(ActionPreviousPage, "< Back", discordgo.SecondaryButton, atFirst),
				w.button(ActionStop, "Stop", discordgo.DangerButton, false),
				w.button(ActionNextPage, "Next >", discordgo.SecondaryButton, atLast),
				w.button(ActionLastPage, "Last", discordgo.SecondaryButton, atLast),
			},
		},
	}
}

func (w *Widget) button(action Action, label string, style discordgo.ButtonStyle, disabled bool) discordgo.MessageComponent {
	return discordgo.Button{
		Label:    label,
		Style:    style,
		Disabled: disabled,
		CustomID: customID(w.id, action),
	}
}

// Serve blocks until the widget stops, times out, or ctx ends.
// Single-page widgets return immediately.
func (w *Widget) Serve(ctx context.Context, d *Dispatcher) error {
	if len(w.Pages) <= 1 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, w.Timeout)
	defer cancel()
	defer d.detach(w.id)

	d.attach(w)

	select {
	case <-ctx.Done():
		return nil
	case err := <-w.done:
		return err
	}
}

func (w *Widget) finish(err error) {
	w.once.Do(func() { w.done <- err })
}

func (w *Widget) move(action Action) {
	switch action {
	case ActionFirstPage:
		w.current = 0
	case ActionPreviousPage:
		if w.current > 0 {
			w.current--
		}
	case ActionNextPage:
		if w.current < len(w.Pages)-1 {
			w.current++
		}
	case ActionLastPage:
		w.current = len(w.Pages) - 1
	}
}

// click applies a button press and always answers the interaction:
// Discord shows an error for clicks left unanswered.
func (w *Widget) click(s *discordgo.Session, in *discordgo.Interaction, action Action) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if action == ActionStop {
		w.finish(nil)
		respondUpdate(s, in, []*discordgo.MessageEmbed{w.Pages[w.current]}, nil)

		return
	}

	w.move(action)

	if w.callback != nil {
		if err := w.callback(action, w.current); err != nil {
			w.finish(err)
			respondEphemeral(s, in, "Something went wrong.")
			return
		}
	}

	if w.current < 0 || w.current >= len(w.Pages) || w.Pages[w.current] == nil {
		_ = s.InteractionRespond(in, &discordgo.InteractionResponse{
			Type: discordgo.InteractionResponseDeferredMessageUpdate,
		})

		return
	}

	respondUpdate(s, in, []*discordgo.MessageEmbed{w.Pages[w.current]}, w.controls())
}

func (w *Widget) checkUser(i *discordgo.InteractionCreate) bool {
	var id string
	if i.Member != nil && i.Member.User != nil {
		id = i.Member.User.ID
	} else if i.User != nil {
		id = i.User.ID
	}

	return id != "" && id == w.author
}

// Dispatcher routes button clicks to live widgets. Register its Handle
// on the session once, then Serve each widget with it.
type Dispatcher struct {
	mu   sync.Mutex
	live map[string]*Widget
}

func NewDispatcher() *Dispatcher {
	return &Dispatcher{live: make(map[string]*Widget)}
}

// Handle routes one component interaction to its widget, answering with
// an ephemeral note when the click isn't actionable.
func (d *Dispatcher) Handle(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i == nil || i.Interaction == nil || i.Type != discordgo.InteractionMessageComponent {
		return
	}

	wid, action, ok := parseCustomID(i.MessageComponentData().CustomID)
	if !ok {
		return
	}

	w, ok := d.lookup(wid)
	if !ok {
		respondEphemeral(s, i.Interaction, "This menu has expired.")
		return
	}

	if !w.checkUser(i) {
		respondEphemeral(s, i.Interaction, "These buttons aren't for you.")
		return
	}

	w.click(s, i.Interaction, action)
}

func (d *Dispatcher) attach(w *Widget) {
	d.mu.Lock()
	defer d.mu.Unlock()

	d.live[w.id] = w
}

func (d *Dispatcher) detach(id string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	delete(d.live, id)
}

func (d *Dispatcher) lookup(id string) (*Widget, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	w, ok := d.live[id]
	return w, ok
}

func customID(wid string, action Action) string {
	return customPrefix + ":" + wid + ":" + action.String()
}

func parseCustomID(id string) (string, Action, bool) {
	parts := strings.Split(id, ":")
	if len(parts) != 4 || parts[0]+":"+parts[1] != customPrefix {
		return "", 0, false
	}

	for a := ActionFirstPage; a <= ActionStop; a++ {
		if parts[3] == a.String() {
			return parts[2], a, true
		}
	}

	return "", 0, false
}

func respondUpdate(s *discordgo.Session, in *discordgo.Interaction, embeds []*discordgo.MessageEmbed, components []discordgo.MessageComponent) {
	_ = s.InteractionRespond(in, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseUpdateMessage,
		Data: &discordgo.InteractionResponseData{
			Embeds:     embeds,
			Components: components,
		},
	})
}

func respondEphemeral(s *discordgo.Session, in *discordgo.Interaction, content string) {
	_ = s.InteractionRespond(in, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Content: content,
			Flags:   discordgo.MessageFlagsEphemeral,
		},
	})
}

func newWidgetID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}

	return hex.EncodeToString(b[:])
}
