package middleware

import (
	"context"
	"time"

	"github.com/VTGare/boe-tea-go/router"
)

// Timeout deadlines ctx.Context(); pass it down so I/O cancels.
// Interaction tokens die after 15 minutes regardless.
func Timeout(d time.Duration) router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(ctx *router.Context) error {
			c, cancel := context.WithTimeout(ctx.Context(), d)
			defer cancel()

			ctx.SetContext(c)
			return next(ctx)
		}
	}
}
