package messages

import (
	"fmt"
	"strings"
	"time"

	"github.com/VTGare/boe-tea-go/internal/arrays"
)

func FormatBool(b bool) string {
	if b {
		return "enabled"
	}

	return "disabled"
}

func ClickHere(url string) string {
	return NamedLink("Click here", url)
}

func NamedLink(name, url string) string {
	return fmt.Sprintf("[%v](%v)", name, url)
}

func LimitExceeded(limit, artworks, count int) string {
	if artworks == 1 {
		return fmt.Sprintf("Album size `(%v)` is higher than the server's limit `(%v)`, album has been cut.", count, limit)
	}

	return fmt.Sprintf("Album size `(%v)` is higher than the server's limit `(%v)`, only the first image of every artwork has been sent.", count, limit)
}

func CrosspostBy(author string) string {
	switch author {
	case "":
		return "Crosspost requested by anonymous"
	default:
		return fmt.Sprintf("Crosspost requested by %v", author)
	}
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

func ListChannels(channels []string) string {
	return strings.Join(
		arrays.Map(
			channels,
			func(s string) string {
				return fmt.Sprintf("<#%v> | `%v`", s, s)
			},
		),
		" • ",
	)
}

// Formats the duration as a combination of human readable time counters
// E.g. `10 * time.Second` will return `10 seconds`
func FormatDuration(d time.Duration) string {
	d = d.Round(1 * time.Second)

	hours := d / time.Hour
	d -= hours * time.Hour

	minutes := d / time.Minute
	d -= minutes * time.Minute

	seconds := d / time.Second
	d -= seconds * time.Second

	sb := strings.Builder{}
	if hours != 0 {
		sb.WriteString(fmt.Sprintf("%02d hours ", hours))
	}
	if minutes != 0 {
		sb.WriteString(fmt.Sprintf("%02d minutes ", minutes))
	}

	sb.WriteString(fmt.Sprintf("%02d seconds", seconds))
	return sb.String()
}

// Returns a Discord relative timestamp
func RelativeTimestamp(t time.Time) string {
	return fmt.Sprintf("<t:%v:R>", t.Unix())
}
