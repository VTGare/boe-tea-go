package dgoutils

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/disgoorg/disgo/bot"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
	"github.com/disgoorg/snowflake/v2"
)

var (
	ErrNotRange    = errors.New("not range")
	ErrRangeSyntax = errors.New("range low is higher than range high")
)

// TrimmerRaw trims <> in case someone wraps the link in it, and characters '!', '@', '#', and '&' for channels and user mentions.
func TrimmerRaw(arg string) string {
	return strings.Trim(arg, "<!@#&>")
}

type Range struct {
	Low  int
	High int
}

func NewRange(s string) (*Range, error) {
	before, after, ok := strings.Cut(s, "-")
	if !ok {
		return nil, ErrNotRange
	}

	lowStr := before
	highStr := after

	low, err := strconv.Atoi(lowStr)
	if err != nil {
		return nil, err
	}

	high, err := strconv.Atoi(highStr)
	if err != nil {
		return nil, err
	}

	if low > high {
		return nil, ErrRangeSyntax
	}

	return &Range{
		Low:  low,
		High: high,
	}, nil
}

func (r *Range) Array() []int {
	arr := make([]int, 0)
	for i := r.Low; i <= r.High; i++ {
		arr = append(arr, i)
	}

	return arr
}

func (r *Range) Map() map[int]struct{} {
	m := make(map[int]struct{})
	for i := r.Low; i <= r.High; i++ {
		m[i] = struct{}{}
	}
	return m
}

// IDString formats a Discord ID the way the store keeps it, with "" for
// none.
func IDString(id snowflake.ID) string {
	if id == 0 {
		return ""
	}

	return id.String()
}

// ParseID is 0 for IDs that don't parse.
func ParseID(s string) snowflake.ID {
	id, _ := snowflake.Parse(s)

	return id
}

// IsForbidden reports whether Discord refused a request with 403. DisGo's
// error text has the JSON error code, not the status.
func IsForbidden(err error) bool {
	var restErr *rest.Error

	return errors.As(err, &restErr) && restErr.Response != nil && restErr.Response.StatusCode == http.StatusForbidden
}

func SendDM(c *bot.Client, userID snowflake.ID, embed discord.Embed) error {
	ch, err := c.Rest.CreateDMChannel(userID)
	if err != nil {
		return fmt.Errorf("failed to create private channel: %w", err)
	}

	if _, err := c.Rest.CreateMessage(ch.ID(), discord.MessageCreate{Embeds: []discord.Embed{embed}}); err != nil {
		return fmt.Errorf("failed to send a direct message: %w", err)
	}

	return nil
}
