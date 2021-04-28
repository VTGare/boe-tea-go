package messages

import (
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/arrays"
)

type Repost struct {
	Title           string
	OriginalMessage string
	ExpiresIn       string
}

type About struct {
	Title         string
	Description   string
	SupportServer string
	InviteLink    string
	Patreon       string
}

func RepostEmbed() Repost {
	return Repost{
		Title:           "Repost detected",
		OriginalMessage: "Original message",
		ExpiresIn:       "Expires in",
	}
}

func FormatBool(b bool) string {
	if b {
		return "enabled"
	}

	return "disabled"
}

func ClickHere(url string) string {
	return fmt.Sprintf("[Click here](%v)", url)
}

func NamedLink(name, url string) string {
	return fmt.Sprintf("[%v](%v)", name, url)
}

func LimitExceeded(limit, count int) string {
	return fmt.Sprintf("Album size `(%v)` is higher than the server's limit `(%v)`, only the first image of every artwork has been sent.", count, limit)
}

func CrosspostBy(author string) string {
	return fmt.Sprintf("Crosspost requested by %v", author)
}

func RateLimit(duration time.Duration) string {
	return fmt.Sprintf("Hold your horses! You're getting rate limited. Try again in **%v**", duration.Round(1*time.Second).String())
}

func NoPerms() string {
	return "You don't have enough permissions to run this command."
}

func NSFWCommand(cmd string) string {
	return fmt.Sprintf("Bonk! You're trying to execute a NSFW command `%v` in a SFW channel.", cmd)
}

func AboutEmbed() About {
	return About{
		Title: "ℹ About",
		Description: fmt.Sprintf(
			"Boe Tea is an ultimate artwork bot for all your artwork related needs. %v\n***%v:***\n%v\n%v\nYou guys are epic!",
			"If you want to copy the invite link, simply right-click it and press Copy Link.",
			"Many thanks to our patron",
			"• Nom | 4 months (Level 1)",
			"• Danyo | 2 months (Level 2)",
		),
		SupportServer: "Support server",
		InviteLink:    "Invite link",
		Patreon:       "Patreon",
	}
}

func ListChannels(channels []string) string {
	return strings.Join(
		arrays.MapString(
			channels,
			func(s string) string {
				return fmt.Sprintf("<#%v> | `%v`", s, s)
			},
		),
		" • ",
	)
}
