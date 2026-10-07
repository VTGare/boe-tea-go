package embeds

import (
	"time"

	"github.com/disgoorg/disgo/discord"
)

const defaultColor = 0x439ef1

type Builder struct {
	embed discord.Embed
}

func NewBuilder() *Builder {
	now := time.Now()

	return &Builder{
		embed: discord.Embed{
			Timestamp: &now,
			Color:     defaultColor,
		},
	}
}

func (eb *Builder) Title(title string) *Builder {
	eb.embed.Title = title
	return eb
}

func (eb *Builder) URL(url string) *Builder {
	eb.embed.URL = url
	return eb
}

func (eb *Builder) Description(desc string) *Builder {
	eb.embed.Description = desc
	return eb
}

func (eb *Builder) AddField(name, value string, inline ...bool) *Builder {
	i := false
	if len(inline) > 0 {
		i = inline[0]
	}

	eb.embed.Fields = append(eb.embed.Fields, discord.EmbedField{Name: name, Value: value, Inline: &i})
	return eb
}

func (eb *Builder) Thumbnail(url string) *Builder {
	eb.embed.Thumbnail = &discord.EmbedResource{URL: url}
	return eb
}

func (eb *Builder) Image(url string) *Builder {
	eb.embed.Image = &discord.EmbedResource{URL: url}
	return eb
}

func (eb *Builder) Author(name, url, icon string) *Builder {
	eb.embed.Author = &discord.EmbedAuthor{Name: name, URL: url, IconURL: icon}
	return eb
}

func (eb *Builder) Color(color int) *Builder {
	eb.embed.Color = color
	return eb
}

// Timestamp clears the timestamp when ts is zero.
func (eb *Builder) Timestamp(ts time.Time) *Builder {
	if ts.IsZero() {
		eb.embed.Timestamp = nil
	} else {
		eb.embed.Timestamp = &ts
	}

	return eb
}

func (eb *Builder) Footer(text, icon string) *Builder {
	eb.embed.Footer = &discord.EmbedFooter{Text: text, IconURL: icon}
	return eb
}

func (eb *Builder) Finalize() discord.Embed {
	return eb.embed
}

func (eb *Builder) ErrorTemplate(message string) *Builder {
	eb.Title("🛑 A wild error appears!").Description(message).Footer("Please use bt!feedback command if something went horribly wrong.", "")
	eb.Color(14555148)
	return eb
}

func (eb *Builder) SuccessTemplate(message string) *Builder {
	eb.Title("✅ Success!").Description(message).Color(6076508)
	return eb
}

func (eb *Builder) FailureTemplate(message string) *Builder {
	eb.Title("❎ Failed!").Description(message).Color(16737650)
	return eb
}

func (eb *Builder) WarnTemplate(message string) *Builder {
	eb.Title("⚠ Warning!").Description(message).Color(16769794)
	return eb
}

func (eb *Builder) InfoTemplate(message string) *Builder {
	eb.Title("ℹ Info").Description(message).Color(defaultColor)
	return eb
}

func (eb *Builder) Clear() *Builder {
	eb.embed = discord.Embed{}
	return eb
}
