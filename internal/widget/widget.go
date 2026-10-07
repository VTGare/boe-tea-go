package widget

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/events"
	"github.com/disgoorg/snowflake/v2"
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
	author snowflake.ID
	// Nil pages aren't loaded yet. The callback can fill them in.
	Pages    []*discord.Embed
	current  int
	callback func(Action, int) error
	Timeout  time.Duration

	mu   sync.Mutex
	id   string
	done chan error
	once sync.Once
}

func New(author snowflake.ID, pages []*discord.Embed) *Widget {
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
func (w *Widget) Controls() []discord.LayoutComponent {
	w.mu.Lock()
	defer w.mu.Unlock()

	return w.controls()
}

func (w *Widget) controls() []discord.LayoutComponent {
	if len(w.Pages) <= 1 {
		return nil
	}

	last := len(w.Pages) - 1
	atFirst := w.current == 0
	atLast := w.current == last

	return []discord.LayoutComponent{
		discord.NewActionRow(
			w.button(ActionFirstPage, "First", discord.ButtonStyleSecondary, atFirst),
			w.button(ActionPreviousPage, "< Back", discord.ButtonStyleSecondary, atFirst),
			w.button(ActionStop, "Stop", discord.ButtonStyleDanger, false),
			w.button(ActionNextPage, "Next >", discord.ButtonStyleSecondary, atLast),
			w.button(ActionLastPage, "Last", discord.ButtonStyleSecondary, atLast),
		),
	}
}

func (w *Widget) button(action Action, label string, style discord.ButtonStyle, disabled bool) discord.InteractiveComponent {
	return discord.NewButton(style, label, customID(w.id, action), "", 0).WithDisabled(disabled)
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
func (w *Widget) click(e *events.ComponentInteractionCreate, action Action) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if action == ActionStop {
		w.finish(nil)
		respondUpdate(e, w.Pages[w.current], []discord.LayoutComponent{})

		return
	}

	w.move(action)

	if w.callback != nil {
		if err := w.callback(action, w.current); err != nil {
			w.finish(err)
			respondEphemeral(e, "Something went wrong.")
			return
		}
	}

	if w.current < 0 || w.current >= len(w.Pages) || w.Pages[w.current] == nil {
		_ = e.DeferUpdateMessage()

		return
	}

	respondUpdate(e, w.Pages[w.current], w.controls())
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
func (d *Dispatcher) Handle(e *events.ComponentInteractionCreate) {
	wid, action, ok := parseCustomID(e.Data.CustomID())
	if !ok {
		return
	}

	w, ok := d.lookup(wid)
	if !ok {
		respondEphemeral(e, "This menu has expired.")
		return
	}

	if e.User().ID != w.author {
		respondEphemeral(e, "These buttons aren't for you.")
		return
	}

	w.click(e, action)
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

// An empty components list removes the buttons.
func respondUpdate(e *events.ComponentInteractionCreate, page *discord.Embed, components []discord.LayoutComponent) {
	update := discord.MessageUpdate{Components: &components}
	if page != nil {
		update.Embeds = &[]discord.Embed{*page}
	}

	_ = e.UpdateMessage(update)
}

// Embeds skips pages that aren't loaded.
func Embeds(pages []*discord.Embed) []discord.Embed {
	out := make([]discord.Embed, 0, len(pages))
	for _, page := range pages {
		if page != nil {
			out = append(out, *page)
		}
	}

	return out
}

func respondEphemeral(e *events.ComponentInteractionCreate, content string) {
	_ = e.CreateMessage(discord.MessageCreate{
		Content: content,
		Flags:   discord.MessageFlagEphemeral,
	})
}

func newWidgetID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}

	return hex.EncodeToString(b[:])
}
