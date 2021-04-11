package messages

import (
	"fmt"
	"time"
)

type Repost struct {
	Title           string
	OriginalMessage string
	ExpiresIn       string
}

func RepostEmbed() *Repost {
	return &Repost{
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
