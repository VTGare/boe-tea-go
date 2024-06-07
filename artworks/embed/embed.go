package embed

import (
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/dgoutils"
	"github.com/VTGare/embeds"
	"github.com/bwmarrin/discordgo"
)

type Embed struct {
	Title       string
	Username    string
	Description string
	FieldName1  string
	FieldValue1 string
	FieldName2  string
	FieldValue2 []string
	Images      []string
	Files       []*discordgo.File
	URL         string
	Timestamp   time.Time
	Footer      string
	AIGenerated bool
}

func (e *Embed) ToEmbed() []*discordgo.MessageSend {
	var (
		length = dgoutils.Ternary(len(e.Files) > 0, 1, len(e.Images))
		pages  = make([]*discordgo.MessageSend, 0, length)
	)

	for i := 0; i < length; i++ {
		eb := embeds.NewBuilder()

		eb.Title(EscapeMarkdown(
			dgoutils.Ternary(length == 1,
				fmt.Sprintf("%v by %v", e.Title, e.Username),
				fmt.Sprintf("%v by %v | Page %v / %v", e.Title, e.Username, i+1, length),
			),
		))

		if len(e.Images) != 0 {
			eb.Image(e.Images[i])
		}

		eb.URL(e.URL)
		eb.Timestamp(e.Timestamp)

		if i < len(e.FieldValue2) {
			eb.AddField(e.FieldName1, e.FieldValue1, true)
			eb.AddField(e.FieldName2, e.FieldValue2[i], true)
		}

		if i == 0 {
			if e.Description != "" {
				eb.Description(EscapeMarkdown(e.Description))
			}

			if e.AIGenerated {
				eb.AddField("⚠️ Disclaimer", "This artwork is AI-generated.")
			}
		}

		if e.Footer != "" {
			eb.Footer(e.Footer, "")
		}

		pages = append(pages, &discordgo.MessageSend{
			Embeds: []*discordgo.MessageEmbed{eb.Finalize()},
			Files:  e.Files,
		})
	}

	return pages
}

func EscapeMarkdown(content string) string {
	markdown := []string{"-", "_", "#", "*", "`", ">"}

	for _, m := range markdown {
		content = strings.ReplaceAll(content, m, "\\"+m)
	}

	return content
}
