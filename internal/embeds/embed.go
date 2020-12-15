package embeds

import (
	"time"

	"github.com/bwmarrin/discordgo"
)

//Builder is a wrapper around DiscordGo MessageEmbed
type Builder struct {
	embed *discordgo.MessageEmbed
}

//NewBuilder returns a new Builder struct with default embed values.
//Timestamp by default is time.Now()
//Color by default is 0x439ef1
func NewBuilder() *Builder {
	return &Builder{
		embed: &discordgo.MessageEmbed{
			Timestamp: time.Now().Format(time.RFC3339),
			Color:     0x439ef1,
		},
	}
}

//Title sets embed's title
func (eb *Builder) Title(title string) *Builder {
	eb.embed.Title = title
	return eb
}

func (eb *Builder) URL(url string) *Builder {
	eb.embed.URL = url
	return eb
}

//Description sets embed's description'
func (eb *Builder) Description(desc string) *Builder {
	eb.embed.Description = desc
	return eb
}

//AddField adds a new field to the embed
func (eb *Builder) AddField(name, value string, inline ...bool) *Builder {
	i := false
	if len(inline) > 0 {
		i = inline[0]
	}

	eb.embed.Fields = append(eb.embed.Fields, &discordgo.MessageEmbedField{Name: name, Value: value, Inline: i})
	return eb
}

//Thumbnail sets embed's thumbnail
func (eb *Builder) Thumbnail(url string) *Builder {
	eb.embed.Thumbnail = &discordgo.MessageEmbedThumbnail{URL: url}
	return eb
}

//Image sets embed's image
func (eb *Builder) Image(url string) *Builder {
	eb.embed.Image = &discordgo.MessageEmbedImage{URL: url}
	return eb
}

//Author sets embed's author
func (eb *Builder) Author(name, url, icon string) *Builder {
	eb.embed.Author = &discordgo.MessageEmbedAuthor{Name: name, URL: url, IconURL: icon}
	return eb
}

//Color sets embed's color
func (eb *Builder) Color(color int) *Builder {
	eb.embed.Color = color
	return eb
}

//Timestamp sets embed's timestamp
func (eb *Builder) Timestamp(ts time.Time) *Builder {
	if ts.IsZero() {
		eb.embed.Timestamp = ""
	} else {
		eb.embed.Timestamp = ts.Format(time.RFC3339)
	}

	return eb
}

func (eb *Builder) TimestampString(ts string) *Builder {
	eb.embed.Timestamp = ts

	return eb
}

//Footer sets embed's footer
func (eb *Builder) Footer(text, icon string) *Builder {
	eb.embed.Footer = &discordgo.MessageEmbedFooter{Text: text, IconURL: icon}
	return eb
}

//Finalize returns a complete DiscordGo embed
func (eb *Builder) Finalize() *discordgo.MessageEmbed {
	return eb.embed
}

//ErrorTemplate retuns an embed built over an error message template
func (eb *Builder) ErrorTemplate(message string) *Builder {
	eb.Title("🛑 A wild error appears!").Description(message).Footer("Please use bt!feedback command if something went horribly wrong.", "")
	eb.Color(14555148)
	return eb
}

//SuccessTemplate retuns an embed built over an success message template
func (eb *Builder) SuccessTemplate(message string) *Builder {
	eb.Title("✅ Success!").Description(message).Color(6076508)
	return eb
}

//FailureTemplate retuns an embed built over an failure message template
func (eb *Builder) FailureTemplate(message string) *Builder {
	eb.Title("❎ Failure!").Description(message).Color(16769794)
	return eb
}

//WarnTemplate retuns an embed built over an warn message template
func (eb *Builder) WarnTemplate(message string) *Builder {
	eb.Title("⚠ Warning!").Description(message).Color(16769794)
	return eb
}

func (eb *Builder) InfoTemplate(message string) *Builder {
	eb.Title("ℹ Info").Description(message).Color(0x439ef1)
	return eb
}

//Clear empties the embed to reuse one builder for several embeds
func (eb *Builder) Clear() *Builder {
	eb.embed = &discordgo.MessageEmbed{}
	return eb
}
