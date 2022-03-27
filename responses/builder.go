package responses

import (
	"github.com/diamondburned/arikawa/v3/api"
	"github.com/diamondburned/arikawa/v3/discord"
	"github.com/diamondburned/arikawa/v3/utils/json/option"
)

type Builder struct {
	r api.InteractionResponseData
}

func (b *Builder) Content(str string) *Builder {
	b.r.Content = option.NewNullableString(str)
	return b
}

func (b *Builder) AddEmbed(embed discord.Embed) *Builder {
	if b.r.Components == nil {
		b.r.Embeds = &[]discord.Embed{embed}
		return b
	}

	embeds := *b.r.Embeds
	embeds = append(embeds, embed)

	b.r.Embeds = &embeds
	return b
}

func (b *Builder) AddComponents(component ...discord.Component) *Builder {
	if b.r.Components == nil {
		b.r.Components = discord.ComponentsPtr(component...)
		return b
	}

	components := *b.r.Components
	components = append(components, *discord.ComponentsPtr(component...)...)

	b.r.Components = &components
	return b
}

func (b *Builder) AddActionRow(component ...discord.InteractiveComponent) *Builder {
	actionRow := discord.ActionRowComponent(component)
	b.AddComponents(&actionRow)
	return b
}

func (b *Builder) AddTextInput(title, customID string, ti *discord.TextInputComponent) *Builder {
	b.r.Title = option.NewNullableString(title)
	b.r.CustomID = option.NewNullableString(customID)
	b.AddComponents(ti)
	return b
}

func (b *Builder) Ephemeral() *Builder {
	b.r.Flags = api.EphemeralResponse
	return b
}

func (b *Builder) Build(t ...api.InteractionResponseType) api.InteractionResponse {
	if len(t) == 0 {
		t = []api.InteractionResponseType{api.MessageInteractionWithSource}
	}

	return api.InteractionResponse{
		Type: t[0],
		Data: &b.r,
	}
}

func LinkButton(label string, url discord.URL) *discord.ButtonComponent {
	return &discord.ButtonComponent{
		Style: discord.LinkButtonStyle(url),
		Label: label,
	}
}

type ButtonOption func(*discord.ButtonComponent)

func Button(label string, customID discord.ComponentID, style discord.ButtonComponentStyle, opts ...ButtonOption) *discord.ButtonComponent {
	button := &discord.ButtonComponent{
		Style:    style,
		Label:    label,
		CustomID: customID,
	}

	for _, opt := range opts {
		opt(button)
	}

	return button
}

func WithEmoji(emoji discord.ComponentEmoji) func(*discord.ButtonComponent) {
	return func(bc *discord.ButtonComponent) {
		bc.Emoji = &emoji
	}
}
