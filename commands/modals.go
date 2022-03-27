package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/arikawautils"
	"github.com/VTGare/boe-tea-go/internal/arikawautils/embeds"
	"github.com/VTGare/boe-tea-go/responses"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
)

type Modal struct{}

var modalHandlers = map[discord.ComponentID]ExecFunc{
	"feedback": feedbackHandler,
	"reply":    replyHandler,
}

func feedbackHandler(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
	var (
		resp api.InteractionResponse
		rb   = responses.Builder{}
		eb   = embeds.NewBuilder()
		mi   = ie.Data.(*discord.ModalInteraction)
	)

	row := mi.Components[0].(*discord.ActionRowComponent)
	ti := (*row)[0].(*discord.TextInputComponent)

	eb.Author(fmt.Sprintf("Feedback from %v", ie.Sender().Tag()), "", ie.Sender().AvatarURL()).
		Description(ti.Value.Val).
		AddField("Author Mention", ie.Sender().Mention(), true).
		AddField("Author ID", ie.SenderID().String(), true)

	if ie.GuildID.IsValid() {
		eb.AddField("Guild", ie.GuildID.String(), true)
	}

	ch, err := s.CreatePrivateChannel(b.Config.Discord.AuthorID)
	if err != nil {
		return resp, err
	}

	if _, err := s.SendEmbeds(ch.ID, eb.Build()); err != nil {
		return resp, err
	}

	success := embeds.NewSuccess("Sent feedback to the dev!").Build()
	return rb.AddEmbed(success).Build(), nil
}

func replyHandler(ctx context.Context, b *bot.Bot, s *state.State, ie discord.InteractionEvent) (api.InteractionResponse, error) {
	var (
		resp api.InteractionResponse
		rb   = responses.Builder{}
		eb   = embeds.NewBuilder()
		mi   = ie.Data.(*discord.ModalInteraction)
	)

	row := mi.Components[0].(*discord.ActionRowComponent)
	ti := (*row)[0].(*discord.TextInputComponent)

	eb.Author(
		fmt.Sprintf("%v's reply", ie.Sender().Tag()),
		"",
		ie.Sender().AvatarURL(),
	).Description(ti.Value.Val)

	sf := strings.TrimPrefix(string(ti.CustomID), "reply:")
	userID, err := arikawautils.UserID(sf)
	if err != nil {
		return resp, err
	}

	ch, err := s.CreatePrivateChannel(userID)
	if err != nil {
		return resp, err
	}

	if _, err := s.SendEmbeds(ch.ID, eb.Build()); err != nil {
		return resp, err
	}

	success := embeds.NewSuccess("Reply successfully sent.").Build()
	return rb.AddEmbed(success).Build(), nil
}

func GetModalHandler(component discord.ComponentID) (ExecFunc, bool) {
	modal, ok := modalHandlers[component]
	return modal, ok
}
