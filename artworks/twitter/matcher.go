package twitter

import (
	"net/url"
	"strconv"
	"strings"

	"github.com/VTGare/boe-tea-go/store"
)

type twitterMatcher struct{}

func (twitterMatcher) Match(s string) (string, bool) {
	u, err := url.ParseRequestURI(s)
	if err != nil {
		return "", false
	}

	var regex = regexp.MustCompile(`^(?:fix(?:v|up))?x\.com|twitter\.com`)
	if !regex.MatchString(u.Host) {
		return "", false
	}

	parts := strings.FieldsFunc(u.Path, func(r rune) bool {
		return r == '/'
	})

	if len(parts) < 3 {
		return "", false
	}

	parts = parts[2:]
	if parts[0] == "status" {
		parts = parts[1:]
	}

	snowflake := parts[0]
	if _, err := strconv.ParseUint(snowflake, 10, 64); err != nil {
		return "", false
	}

	return snowflake, true
}

func (twitterMatcher) Enabled(g *store.Guild) bool {
	return g.Twitter
}
