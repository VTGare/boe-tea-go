package commands

import (
	"github.com/VTGare/boe-tea-go/bot"
	"github.com/VTGare/boe-tea-go/internal/arikawautils/embeds"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/state"
)

func ping(b *bot.Bot, s *state.State) (api.InteractionResponse, error) {
	eb := embeds.NewBuilder()

	eb.Title("🏓 Pong!")
	//eb.AddField("Latency", latency.Round(time.Millisecond).String())

	return api.InteractionResponse{
		Data: &api.InteractionResponseData{
			Embeds: &[]discord.Embed{eb.Build()},
		},
		Type: api.MessageInteractionWithSource,
	}, nil
}
