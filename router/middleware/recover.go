package middleware

import (
	"runtime/debug"

	"github.com/VTGare/boe-tea-go/router"
)

// Recover turns panics into *router.PanicError, flowing through error
// handling (and middleware registered before it, like Logging).
func Recover() router.Middleware {
	return func(next router.Handler) router.Handler {
		return func(ctx *router.Context) (err error) {
			defer func() {
				if rec := recover(); rec != nil {
					err = &router.PanicError{Value: rec, Stack: debug.Stack()}
				}
			}()

			return next(ctx)
		}
	}
}
