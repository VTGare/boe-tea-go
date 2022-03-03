package embeds

import (
	"time"

	"github.com/diamondburned/arikawa/v3/discord"
)

type Builder struct {
	e *discord.Embed
}

const (
	ColorBlack  discord.Color = 0
	ColorWhite  discord.Color = 0xFFFFFF
	ColorPurple discord.Color = 0xc687bb
	ColorBlue   discord.Color = 0x60B0E0
	ColorGreen  discord.Color = 0x228B22
	ColorRed    discord.Color = 0xD92121
	ColorYellow discord.Color = 0xFAFA37
)

func NewBuilder() *Builder {
	return &Builder{
		e: &discord.Embed{
			Type:  discord.NormalEmbed,
			Color: ColorPurple,
		},
	}
}

func NewBuilderFromEmbed(e discord.Embed) *Builder {
	return &Builder{
		e: &e,
	}
}

func (b *Builder) Title(title string) *Builder {
	b.e.Title = title

	return b
}

func (b *Builder) Description(desc string) *Builder {
	b.e.Description = desc
	return b
}

func (b *Builder) AddField(name, val string, inline ...bool) *Builder {
	var isInline bool
	if len(inline) > 0 {
		isInline = inline[0]
	}

	b.e.Fields = append(b.e.Fields, discord.EmbedField{
		Name:   name,
		Value:  val,
		Inline: isInline,
	})

	return b
}

func (b *Builder) AddFields(fields ...discord.EmbedField) *Builder {
	b.e.Fields = append(b.e.Fields, fields...)
	return b
}

func (b *Builder) Image(url string) *Builder {
	b.e.Image = &discord.EmbedImage{
		URL: url,
	}
	return b
}

func (b *Builder) Thumbnail(url string) *Builder {
	b.e.Thumbnail = &discord.EmbedThumbnail{
		URL: url,
	}
	return b
}

func (b *Builder) Color(color discord.Color) *Builder {
	b.e.Color = color
	return b
}

func (b *Builder) URL(url string) *Builder {
	b.e.URL = url
	return b
}

func (b *Builder) Footer(text, icon string) *Builder {
	b.e.Footer = &discord.EmbedFooter{
		Text: text,
		Icon: icon,
	}
	return b
}

func (b *Builder) Timestamp(t time.Time) *Builder {
	b.e.Timestamp = discord.NewTimestamp(t)
	return b
}

func (b *Builder) TimestampRFC3339(s string) *Builder {
	ts, _ := time.Parse(time.RFC3339, s)
	return b.Timestamp(ts)
}

func (b *Builder) Build() discord.Embed {
	return *b.e
}
