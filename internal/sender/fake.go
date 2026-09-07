package sender

import (
	"fmt"
	"sync"
	"time"

	"github.com/bwmarrin/discordgo"
)

// SentComplex records one SendComplex call.
type SentComplex struct {
	GuildID   string
	ChannelID string
	Message   *discordgo.MessageSend
	Sent      *discordgo.Message
}

// SentEmbed records one SendEmbed call.
type SentEmbed struct {
	GuildID   string
	ChannelID string
	Embed     *discordgo.MessageEmbed
	Sent      *discordgo.Message
}

// DeletedMessage records one DeleteMessage call.
type DeletedMessage struct {
	GuildID   string
	ChannelID string
	MessageID string
}

// AddedReaction records one AddReaction call.
type AddedReaction struct {
	GuildID   string
	ChannelID string
	MessageID string
	Emoji     string
}

// EditedEmbed records one EditEmbed call.
type EditedEmbed struct {
	GuildID   string
	ChannelID string
	MessageID string
	Embed     *discordgo.MessageEmbed
}

// RemovedReaction records one RemoveReaction call.
type RemovedReaction struct {
	GuildID   string
	ChannelID string
	MessageID string
	Emoji     string
	UserID    string
}

// FakeSender is the test Sender adapter. It records every call behind
// a mutex and answers from its configurable fields. Use NewFake so
// permission checks default to allowed.
type FakeSender struct {
	mu sync.Mutex

	Complex   []SentComplex
	Embeds    []SentEmbed
	Deleted   []DeletedMessage
	Reactions []AddedReaction
	Edited    []EditedEmbed
	Unreacted []RemovedReaction
	Cleared   []DeletedMessage
	Expired   []*discordgo.Message

	ChannelPerms    bool
	ChannelPermsErr error
	GuildPerms      bool
	GuildPermsErr   error

	SendErr   error
	DeleteErr error
	ReactErr  error

	// Skip makes sends fail with ErrSkipped, exercising the
	// permission-skip path without touching permission flags.
	Skip bool

	counter int
}

// NewFake returns a FakeSender that allows every permission check.
func NewFake() *FakeSender {
	return &FakeSender{
		ChannelPerms: true,
		GuildPerms:   true,
	}
}

func (f *FakeSender) nextMessage(guildID, channelID string) *discordgo.Message {
	f.counter++

	return &discordgo.Message{
		ID:        fmt.Sprintf("fake-%d", f.counter),
		ChannelID: channelID,
		GuildID:   guildID,
	}
}

func (f *FakeSender) SendComplex(guildID, channelID string, message *discordgo.MessageSend) (*discordgo.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.SendErr != nil {
		return nil, f.SendErr
	}

	if f.Skip {
		return nil, ErrSkipped
	}

	f.Complex = append(f.Complex, SentComplex{
		GuildID:   guildID,
		ChannelID: channelID,
		Message:   message,
		Sent:      f.nextMessage(guildID, channelID),
	})

	return f.Complex[len(f.Complex)-1].Sent, nil
}

func (f *FakeSender) SendEmbed(guildID, channelID string, embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.SendErr != nil {
		return nil, f.SendErr
	}

	if f.Skip {
		return nil, ErrSkipped
	}

	f.Embeds = append(f.Embeds, SentEmbed{
		GuildID:   guildID,
		ChannelID: channelID,
		Embed:     embed,
		Sent:      f.nextMessage(guildID, channelID),
	})

	return f.Embeds[len(f.Embeds)-1].Sent, nil
}

func (f *FakeSender) DeleteMessage(guildID, channelID, messageID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Deleted = append(f.Deleted, DeletedMessage{
		GuildID:   guildID,
		ChannelID: channelID,
		MessageID: messageID,
	})

	return f.DeleteErr
}

func (f *FakeSender) AddReaction(guildID, channelID, messageID, emoji string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Reactions = append(f.Reactions, AddedReaction{
		GuildID:   guildID,
		ChannelID: channelID,
		MessageID: messageID,
		Emoji:     emoji,
	})

	return f.ReactErr
}

func (f *FakeSender) EditEmbed(guildID, channelID, messageID string, embed *discordgo.MessageEmbed) (*discordgo.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.SendErr != nil {
		return nil, f.SendErr
	}

	f.Edited = append(f.Edited, EditedEmbed{
		GuildID:   guildID,
		ChannelID: channelID,
		MessageID: messageID,
		Embed:     embed,
	})

	return f.nextMessage(guildID, channelID), nil
}

func (f *FakeSender) RemoveReaction(guildID, channelID, messageID, emoji, userID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Unreacted = append(f.Unreacted, RemovedReaction{
		GuildID:   guildID,
		ChannelID: channelID,
		MessageID: messageID,
		Emoji:     emoji,
		UserID:    userID,
	})

	return f.ReactErr
}

func (f *FakeSender) RemoveAllReactions(guildID, channelID, messageID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Cleared = append(f.Cleared, DeletedMessage{
		GuildID:   guildID,
		ChannelID: channelID,
		MessageID: messageID,
	})

	return f.ReactErr
}

func (f *FakeSender) HasChannelPerms(_, _ string, _ int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.ChannelPerms, f.ChannelPermsErr
}

func (f *FakeSender) BotHasGuildPerms(_ string, _ int64) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.GuildPerms, f.GuildPermsErr
}

func (f *FakeSender) Expire(message *discordgo.Message, _ ...time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Expired = append(f.Expired, message)
}

// ReactionsAdded returns a copy of the recorded AddReaction calls.
func (f *FakeSender) ReactionsAdded() []AddedReaction {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]AddedReaction(nil), f.Reactions...)
}

// EmbedsSent returns a copy of the recorded SendEmbed calls.
func (f *FakeSender) EmbedsSent() []SentEmbed {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]SentEmbed(nil), f.Embeds...)
}

// ComplexSent returns a copy of the recorded SendComplex calls.
func (f *FakeSender) ComplexSent() []SentComplex {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]SentComplex(nil), f.Complex...)
}

// EmbedsEdited returns a copy of the recorded EditEmbed calls.
func (f *FakeSender) EmbedsEdited() []EditedEmbed {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]EditedEmbed(nil), f.Edited...)
}

// ReactionsRemoved returns a copy of the recorded RemoveReaction calls.
func (f *FakeSender) ReactionsRemoved() []RemovedReaction {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]RemovedReaction(nil), f.Unreacted...)
}

// ReactionsCleared returns a copy of the recorded RemoveAllReactions calls.
func (f *FakeSender) ReactionsCleared() []DeletedMessage {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]DeletedMessage(nil), f.Cleared...)
}
