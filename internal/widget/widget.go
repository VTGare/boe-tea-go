// Package widget owns reaction-paginated embeds. Callers send pages
// through the Sender and drive them with reactions.
package widget

import (
	"context"
	"time"

	"github.com/VTGare/boe-tea-go/internal/sender"
	"github.com/bwmarrin/discordgo"
)

// DefaultTimeout bounds one widget run.
const DefaultTimeout = 2 * time.Minute

// Action is one pagination control.
type Action int

const (
	ActionFirstPage Action = iota
	ActionFiveDown
	ActionPreviousPage
	ActionStop
	ActionNextPage
	ActionFiveUp
	ActionLastPage
)

var actionMap = map[string]Action{
	"⏮": ActionFirstPage,
	"⏪": ActionFiveDown,
	"◀": ActionPreviousPage,
	"⏹": ActionStop,
	"▶": ActionNextPage,
	"⏩": ActionFiveUp,
	"⏭": ActionLastPage,
}

func (a Action) String() string {
	icons := []string{"⏮", "⏪", "◀", "⏹", "▶", "⏩", "⏭"}
	if int(a) < 0 || int(a) >= len(icons) {
		return ""
	}

	return icons[a]
}

// Widget is one paginated message run.
type Widget struct {
	sender   sender.Sender
	events   EventSource
	guildID  string
	author   string
	Pages    []*discordgo.MessageEmbed
	current  int
	callback func(Action, int) error
	Timeout  time.Duration
}

// New builds a widget.
func New(s sender.Sender, events EventSource, guildID, author string, pages []*discordgo.MessageEmbed) *Widget {
	return &Widget{
		sender:  s,
		events:  events,
		guildID: guildID,
		author:  author,
		Pages:   pages,
		Timeout: DefaultTimeout,
	}
}

// WithCallback runs fn after every handled control, before the page edit.
func (w *Widget) WithCallback(fn func(Action, int) error) {
	w.callback = fn
}

// Start sends the first page, arms the controls, and runs until stop,
// timeout, or cancellation. Single-page widgets send and return.
func (w *Widget) Start(ctx context.Context, channelID string) error {
	if len(w.Pages) == 0 {
		return nil
	}

	ctx, cancel := context.WithTimeout(ctx, w.Timeout)
	defer cancel()

	events, err := w.events.Watch(ctx)
	if err != nil {
		return err
	}

	msg, err := w.sender.SendEmbed(w.guildID, channelID, w.Pages[0])
	if err != nil {
		return err
	}

	if w.len() == 1 {
		return nil
	}

	controls := []string{"◀", "⏹", "▶"}
	if w.len() > 5 {
		controls = []string{"⏮", "⏪", "◀", "⏹", "▶", "⏩", "⏭"}
	}

	for _, emoji := range controls {
		_ = w.sender.AddReaction(w.guildID, msg.ChannelID, msg.ID, emoji)
	}

	self := w.events.SelfID()

	for {
		select {
		case <-ctx.Done():
			return nil
		case e, ok := <-events:
			if !ok {
				return nil
			}

			stop, err := w.step(msg, self, e)
			if err != nil {
				return err
			}

			if stop {
				return nil
			}
		}
	}
}

// step handles one reaction, reporting whether the run ends.
func (w *Widget) step(msg *discordgo.Message, self string, e *discordgo.MessageReactionAdd) (bool, error) {
	if e == nil || e.MessageReaction == nil {
		return false, nil
	}

	reaction := e.MessageReaction
	if reaction.MessageID != msg.ID {
		return false, nil
	}

	if reaction.UserID == self && self != "" {
		return false, nil
	}

	if reaction.UserID != w.author {
		return false, nil
	}

	action, ok := actionMap[reaction.Emoji.APIName()]
	if !ok {
		return false, nil
	}

	if action == ActionStop {
		_ = w.sender.RemoveAllReactions(w.guildID, msg.ChannelID, msg.ID)

		return true, nil
	}

	w.move(action)

	if w.callback != nil {
		if err := w.callback(action, w.current); err != nil {
			return false, err
		}
	}

	if w.current < 0 || w.current >= w.len() || w.Pages[w.current] == nil {
		return false, nil
	}

	if _, err := w.sender.EditEmbed(w.guildID, msg.ChannelID, msg.ID, w.Pages[w.current]); err != nil {
		return false, err
	}

	_ = w.sender.RemoveReaction(w.guildID, msg.ChannelID, msg.ID, reaction.Emoji.APIName(), reaction.UserID)

	return false, nil
}

func (w *Widget) move(action Action) {
	switch action {
	case ActionFirstPage:
		w.current = 0
	case ActionFiveDown:
		w.current -= 5
		if w.current < 0 {
			w.current = 0
		}
	case ActionPreviousPage:
		if w.current > 0 {
			w.current--
		}
	case ActionNextPage:
		if w.current < w.len()-1 {
			w.current++
		}
	case ActionFiveUp:
		w.current += 5
		if w.current >= w.len() {
			w.current = w.len() - 1
		}
	case ActionLastPage:
		w.current = w.len() - 1
	}
}

func (w *Widget) len() int {
	return len(w.Pages)
}
