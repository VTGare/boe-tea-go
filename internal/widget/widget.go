package widget

import (
	"time"

	"github.com/bwmarrin/discordgo"
)

var (
	controls = map[string]bool{"⏮": true, "⏪": true, "◀": true, "⏹": true, "▶": true, "⏩": true, "⏭": true}
)

//Widget is an interactive DiscordGo widget interface
type Widget struct {
	s           *discordgo.Session
	m           *discordgo.Message
	author      string
	currentPage int
	Pages       []*discordgo.MessageEmbed
}

func NewWidget(s *discordgo.Session, author string, embeds []*discordgo.MessageEmbed) *Widget {
	return &Widget{s, nil, author, 0, embeds}
}

func (w *Widget) Start(channelID string) error {
	m, err := w.s.ChannelMessageSendEmbed(channelID, w.Pages[0])
	if err != nil {
		return err
	}
	w.m = m

	if w.len() > 1 {
		w.s.MessageReactionAdd(m.ChannelID, m.ID, "⏮")
		w.s.MessageReactionAdd(m.ChannelID, m.ID, "⏪")
		w.s.MessageReactionAdd(m.ChannelID, m.ID, "◀")
		w.s.MessageReactionAdd(m.ChannelID, m.ID, "⏹")
		w.s.MessageReactionAdd(m.ChannelID, m.ID, "▶")
		w.s.MessageReactionAdd(m.ChannelID, m.ID, "⏩")
		w.s.MessageReactionAdd(m.ChannelID, m.ID, "⏭")

		var reaction *discordgo.MessageReaction
		for {
			select {
			case k := <-nextMessageReactionAdd(w.s):
				reaction = k.MessageReaction
			case <-time.After(2 * time.Minute):
				return nil
			}

			r := reaction.Emoji.APIName()
			_, ok := controls[r]
			if !ok {
				continue
			}

			if reaction.MessageID != w.m.ID || w.s.State.User.ID == reaction.UserID || reaction.UserID != w.author {
				continue
			}

			switch reaction.Emoji.APIName() {
			case "⏮":
				err := w.firstPage()
				w.s.MessageReactionRemove(w.m.ChannelID, w.m.ID, reaction.Emoji.APIName(), reaction.UserID)
				if err != nil {
					return err
				}
			case "⏪":
				err := w.fivePagesDown()
				w.s.MessageReactionRemove(w.m.ChannelID, w.m.ID, reaction.Emoji.APIName(), reaction.UserID)
				if err != nil {
					return err
				}
			case "◀":
				err := w.pageDown()
				w.s.MessageReactionRemove(w.m.ChannelID, w.m.ID, reaction.Emoji.APIName(), reaction.UserID)
				if err != nil {
					return err
				}
			case "⏹":
				w.s.MessageReactionsRemoveAll(w.m.ChannelID, w.m.ID)
				return nil
			case "▶":
				err := w.pageUp()
				w.s.MessageReactionRemove(w.m.ChannelID, w.m.ID, reaction.Emoji.APIName(), reaction.UserID)
				if err != nil {
					return err
				}
			case "⏩":
				err := w.fivePagesUp()
				w.s.MessageReactionRemove(w.m.ChannelID, w.m.ID, reaction.Emoji.APIName(), reaction.UserID)
				if err != nil {
					return err
				}
			case "⏭":
				err := w.lastPage()
				w.s.MessageReactionRemove(w.m.ChannelID, w.m.ID, reaction.Emoji.APIName(), reaction.UserID)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (w *Widget) pageUp() error {
	if w.currentPage == w.len()-1 || w.len() <= 1 {
		return nil
	}

	w.currentPage++
	_, err := w.s.ChannelMessageEditEmbed(w.m.ChannelID, w.m.ID, w.Pages[w.currentPage])
	if err != nil {
		return err
	}

	return nil
}

func (w *Widget) fivePagesUp() error {
	if w.currentPage == w.len()-1 || w.len() <= 1 {
		return nil
	}

	w.currentPage += 5
	if w.currentPage >= w.len() {
		w.currentPage = w.len() - 1
	}

	_, err := w.s.ChannelMessageEditEmbed(w.m.ChannelID, w.m.ID, w.Pages[w.currentPage])
	if err != nil {
		return err
	}

	return nil
}

func (w *Widget) pageDown() error {
	if w.currentPage == 0 || w.len() <= 1 {
		return nil
	}

	w.currentPage--
	_, err := w.s.ChannelMessageEditEmbed(w.m.ChannelID, w.m.ID, w.Pages[w.currentPage])
	if err != nil {
		return err
	}

	return nil
}

func (w *Widget) fivePagesDown() error {
	if w.currentPage == 0 || w.len() <= 1 {
		return nil
	}

	w.currentPage -= 5
	if w.currentPage < 0 {
		w.currentPage = 0
	}

	_, err := w.s.ChannelMessageEditEmbed(w.m.ChannelID, w.m.ID, w.Pages[w.currentPage])
	if err != nil {
		return err
	}

	return nil
}

func (w *Widget) lastPage() error {
	if w.currentPage == w.len()-1 || w.len() <= 1 {
		return nil
	}

	w.currentPage = w.len() - 1
	_, err := w.s.ChannelMessageEditEmbed(w.m.ChannelID, w.m.ID, w.Pages[w.currentPage])
	if err != nil {
		return err
	}

	return nil
}

func (w *Widget) firstPage() error {
	if w.currentPage == 0 || w.len() <= 1 {
		return nil
	}

	w.currentPage = 0
	_, err := w.s.ChannelMessageEditEmbed(w.m.ChannelID, w.m.ID, w.Pages[w.currentPage])
	if err != nil {
		return err
	}

	return nil
}

func (w *Widget) len() int {
	return len(w.Pages)
}

func nextMessageReactionAdd(s *discordgo.Session) chan *discordgo.MessageReactionAdd {
	out := make(chan *discordgo.MessageReactionAdd)
	s.AddHandlerOnce(func(_ *discordgo.Session, e *discordgo.MessageReactionAdd) {
		out <- e
	})
	return out
}
