package middleware

import (
	"context"
	"time"

	"github.com/VTGare/boe-tea-go/router"
)

// Timeout puts a deadline on ctx.Context(). Pass that context to I/O so
// it gets cancelled. Interaction tokens expire after 15 minutes anyway.
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
