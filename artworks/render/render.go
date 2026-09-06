// Package render owns artwork presentation.
package render

import (
	"fmt"
	"strings"

	"github.com/VTGare/boe-tea-go/artworks"
	"github.com/VTGare/embeds"
	"github.com/bwmarrin/discordgo"
)

const (
	pageSuffix   = " | Page %v / %v"
	tagsHeader   = "**Tags:**"
	aiFieldName  = "⚠️ Disclaimer"
	aiFieldValue = "This artwork is AI-generated."
)

// Options carries the per-send presentation inputs. Everything in here
// varies per guild or per event, never per provider.
type Options struct {
	TagsEnabled   bool
	Crosspost     bool
	AuthorName    string
	AuthorIconURL string
	Reference     *discordgo.MessageReference
}

// Input is one artwork awaiting rendering.
type Input struct {
	ID            string
	Footer        string
	Rendered      artworks.Rendered
	SkipFirstPage bool
}

// Bundle pairs sent messages with the artwork they came from.
type Bundle struct {
	ID    string
	Sends []*discordgo.MessageSend
}

// Build renders every input into a Bundle.
func Build(inputs []Input, opts Options) []Bundle {
	bundles := make([]Bundle, 0, len(inputs))

	for _, in := range inputs {
		sends := pages(in.Rendered, in.Footer, opts)

		if in.SkipFirstPage && len(sends) > 0 {
			sends = sends[1:]
		}

		if len(sends) == 0 {
			continue
		}

		decorate(sends, opts)
		bundles = append(bundles, Bundle{ID: in.ID, Sends: sends})
	}

	return bundles
}

func pages(r artworks.Rendered, footer string, opts Options) []*discordgo.MessageSend {
	count := max(len(r.Images), 1)

	sends := make([]*discordgo.MessageSend, 0, count)
	for ind := range count {
		eb := embeds.NewBuilder()
		eb.Title(pageTitle(r, ind)).URL(r.URL).Timestamp(r.Timestamp)

		if ind == 0 {
			firstPage(eb, r, footer, opts.TagsEnabled)
		} else if footer != "" {
			eb.Footer(footer, "")
		}

		if ind < len(r.Images) {
			pageImage(eb, r.Images[ind])
		}

		send := &discordgo.MessageSend{Embeds: []*discordgo.MessageEmbed{eb.Finalize()}}

		if ind == 0 {
			send.Files = r.Files
		}

		sends = append(sends, send)
	}

	return sends
}

func pageTitle(r artworks.Rendered, ind int) string {
	if len(r.Images) > 1 {
		return r.Title + fmt.Sprintf(pageSuffix, ind+1, len(r.Images))
	}

	return r.Title
}

func firstPage(eb *embeds.Builder, r artworks.Rendered, footer string, tagsEnabled bool) {
	for _, field := range r.Fields {
		eb.AddField(field.Name, field.Value, field.Inline)
	}

	if desc := description(r, tagsEnabled); desc != "" {
		eb.Description(desc)
	}

	if footer != "" {
		eb.Footer(footer, "")
	}

	if r.AIGenerated {
		eb.AddField(aiFieldName, aiFieldValue)
	}
}

func pageImage(eb *embeds.Builder, image artworks.RenderedImage) {
	if image.Preview != "" {
		eb.Image(image.Preview)
	}

	if image.Original != "" {
		eb.AddField("Original quality", fmt.Sprintf("[Click here](%v)", image.Original), true)
	}
}

// description composes the first-page description from the artwork text
// and its tag block.
func description(r artworks.Rendered, tagsEnabled bool) string {
	desc := r.Description

	if tagsEnabled && len(r.Tags) > 0 {
		tags := make([]string, 0, len(r.Tags))
		for _, tag := range r.Tags {
			if r.TagLinkTemplate != "" {
				tags = append(tags, fmt.Sprintf(r.TagLinkTemplate, tag, tag))
			} else {
				tags = append(tags, tag)
			}
		}

		block := fmt.Sprintf("%v\n%v", tagsHeader, strings.Join(tags, " • "))
		if desc == "" {
			return block
		}

		return fmt.Sprintf("%v\n\n%v", desc, block)
	}

	return desc
}

// decorate stamps every page with the crosspost author or the reply
// reference.
func decorate(sends []*discordgo.MessageSend, opts Options) {
	for _, send := range sends {
		if send == nil || len(send.Embeds) == 0 || send.Embeds[0] == nil {
			continue
		}

		if opts.Crosspost {
			if opts.AuthorName != "" {
				send.Embeds[0].Author = &discordgo.MessageEmbedAuthor{
					Name:    opts.AuthorName,
					IconURL: opts.AuthorIconURL,
				}
			}
		} else if opts.Reference != nil {
			send.AllowedMentions = &discordgo.MessageAllowedMentions{}
			send.Reference = opts.Reference
		}
	}
}
