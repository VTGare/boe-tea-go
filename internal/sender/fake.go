package sender

import (
	"fmt"
	"sync"
	"time"

	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/snowflake/v2"
)

// SentComplex records one SendComplex call.
type SentComplex struct {
	ChannelID snowflake.ID
	Message   discord.MessageCreate
	Sent      *discord.Message
}

// SentEmbed records one SendEmbed call.
type SentEmbed struct {
	ChannelID snowflake.ID
	Embed     discord.Embed
	Sent      *discord.Message
}

// DeletedMessage records one DeleteMessage call.
type DeletedMessage struct {
	ChannelID snowflake.ID
	MessageID snowflake.ID
}

// AddedReaction records one AddReaction call.
type AddedReaction struct {
	ChannelID snowflake.ID
	MessageID snowflake.ID
	Emoji     string
}

// MemberKey identifies one guild membership.
type MemberKey struct {
	GuildID snowflake.ID
	UserID  snowflake.ID
}

// FakeSender is a Sender for tests. It records every call and answers
// lookups from its fields. Create it with NewFake so permission checks
// pass by default.
type FakeSender struct {
	mu sync.Mutex

	Complex   []SentComplex
	Embeds    []SentEmbed
	Deleted   []DeletedMessage
	Reactions []AddedReaction
	Expired   []*discord.Message

	ChannelPerms    bool
	ChannelPermsErr error
	GuildPerms      bool
	GuildPermsErr   error

	// ChannelGuilds answers ChannelGuildID lookups. Missing channels
	// report an error unless ChannelErr is set.
	ChannelGuilds map[snowflake.ID]snowflake.ID
	ChannelErr    error

	// Members answers IsMember lookups. Absent keys report non-members.
	Members   map[MemberKey]bool
	MemberErr error

	SendErr   error
	DeleteErr error
	ReactErr  error

	// Skip makes every send return ErrSkipped, as if the bot lacked
	// permissions.
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

// Sent messages get IDs 1, 2, 3 and so on.
func (f *FakeSender) nextMessage(channelID snowflake.ID) *discord.Message {
	f.counter++

	return &discord.Message{
		ID:        snowflake.ID(f.counter),
		ChannelID: channelID,
	}
}

func (f *FakeSender) SendComplex(channelID snowflake.ID, message discord.MessageCreate) (*discord.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.SendErr != nil {
		return nil, f.SendErr
	}

	if f.Skip {
		return nil, ErrSkipped
	}

	f.Complex = append(f.Complex, SentComplex{
		ChannelID: channelID,
		Message:   message,
		Sent:      f.nextMessage(channelID),
	})

	return f.Complex[len(f.Complex)-1].Sent, nil
}

func (f *FakeSender) SendEmbed(channelID snowflake.ID, embed discord.Embed) (*discord.Message, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.SendErr != nil {
		return nil, f.SendErr
	}

	if f.Skip {
		return nil, ErrSkipped
	}

	f.Embeds = append(f.Embeds, SentEmbed{
		ChannelID: channelID,
		Embed:     embed,
		Sent:      f.nextMessage(channelID),
	})

	return f.Embeds[len(f.Embeds)-1].Sent, nil
}

func (f *FakeSender) DeleteMessage(channelID, messageID snowflake.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Deleted = append(f.Deleted, DeletedMessage{
		ChannelID: channelID,
		MessageID: messageID,
	})

	return f.DeleteErr
}

func (f *FakeSender) AddReaction(channelID, messageID snowflake.ID, emoji string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.Reactions = append(f.Reactions, AddedReaction{
		ChannelID: channelID,
		MessageID: messageID,
		Emoji:     emoji,
	})

	return f.ReactErr
}

func (f *FakeSender) HasChannelPerms(_, _ snowflake.ID, _ discord.Permissions) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.ChannelPerms, f.ChannelPermsErr
}

func (f *FakeSender) ChannelGuildID(channelID snowflake.ID) (snowflake.ID, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.ChannelErr != nil {
		return 0, f.ChannelErr
	}

	guildID, ok := f.ChannelGuilds[channelID]
	if !ok {
		return 0, fmt.Errorf("sender: unknown test channel %v", channelID)
	}

	return guildID, nil
}

func (f *FakeSender) IsMember(guildID, userID snowflake.ID) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.MemberErr != nil {
		return false, f.MemberErr
	}

	return f.Members[MemberKey{GuildID: guildID, UserID: userID}], nil
}

func (f *FakeSender) BotHasGuildPerms(_ snowflake.ID, _ discord.Permissions) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	return f.GuildPerms, f.GuildPermsErr
}

func (f *FakeSender) Expire(message *discord.Message, _ ...time.Duration) {
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
