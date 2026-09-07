package widget

import (
	"context"
	"fmt"

	"github.com/bwmarrin/discordgo"
)

// EventSource streams message reaction adds.
type EventSource interface {
	// Watch returns the stream, closed when ctx ends.
	Watch(ctx context.Context) (<-chan *discordgo.MessageReactionAdd, error)

	// SelfID is the bot's own user ID, used to ignore its control
	// reactions.
	SelfID() string
}

type sessionSource struct {
	session *discordgo.Session
}

// NewSessionSource builds the production EventSource.
func NewSessionSource(s *discordgo.Session) EventSource {
	return &sessionSource{session: s}
}

func (ss *sessionSource) Watch(ctx context.Context) (<-chan *discordgo.MessageReactionAdd, error) {
	if ss.session == nil {
		return nil, fmt.Errorf("discord session not ready")
	}

	stream := make(chan *discordgo.MessageReactionAdd)
	remove := ss.session.AddHandler(func(_ *discordgo.Session, e *discordgo.MessageReactionAdd) {
		select {
		case stream <- e:
		case <-ctx.Done():
		}
	})

	go func() {
		<-ctx.Done()
		remove()
	}()

	return stream, nil
}

func (ss *sessionSource) SelfID() string {
	if ss.session == nil || ss.session.State == nil || ss.session.State.User == nil {
		return ""
	}

	return ss.session.State.User.ID
}

// FakeSource is the test EventSource.
type FakeSource struct {
	Ch     chan *discordgo.MessageReactionAdd
	Err    error
	BotID  string
	closed bool
}

// NewFakeSource returns a FakeSource with a buffered stream.
func NewFakeSource() *FakeSource {
	return &FakeSource{Ch: make(chan *discordgo.MessageReactionAdd, 8)}
}

func (f *FakeSource) Watch(context.Context) (<-chan *discordgo.MessageReactionAdd, error) {
	if f.Err != nil {
		return nil, f.Err
	}

	return f.Ch, nil
}

func (f *FakeSource) SelfID() string {
	return f.BotID
}

// Emit queues one reaction add from userID on messageID.
func (f *FakeSource) Emit(messageID, userID, emoji string) {
	f.Ch <- &discordgo.MessageReactionAdd{MessageReaction: &discordgo.MessageReaction{
		MessageID: messageID,
		UserID:    userID,
		Emoji:     discordgo.Emoji{Name: emoji},
	}}
}

// Close ends the stream, ending any running widget.
func (f *FakeSource) Close() {
	if !f.closed {
		f.closed = true
		close(f.Ch)
	}
}
