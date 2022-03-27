package responses

import (
	"github.com/VTGare/boe-tea-go/internal/arikawautils/embeds"
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
)

var (
	GuildOnly               = FromEmbed(embeds.NewFail("This command can't be used in direct messages.").Build())
	AuthorOnly              = FromEmbed(embeds.NewFail("This command can only be used by the developer.").Build())
	InsufficientPermissions = FromEmbed(embeds.NewFail("Insufficient permissions to execute this command.").Build())
	InternalError           = func(err error) api.InteractionResponse {
		return api.InteractionResponse{
			Data: &api.InteractionResponseData{
				Embeds: &[]discord.Embed{embeds.NewFail("Internal error occured").
					AddField("Error", err.Error()).
					Build(),
				},
			},
		}
	}
)

func FromEmbed(embed discord.Embed) api.InteractionResponse {
	return api.InteractionResponse{
		Data: &api.InteractionResponseData{
			Embeds: &[]discord.Embed{
				embed,
			},
		},
		Type: api.MessageInteractionWithSource,
	}
}
