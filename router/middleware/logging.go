// Package middleware provides common middleware for router.Router.
//
// Recommended order (outermost first):
//
//	r.Use(
//		middleware.Logging(log), // sees everything, including panics turned into errors
//		middleware.Recover(),    // converts panics into *router.PanicError
//		middleware.Timeout(time.Minute),
//	)
package middleware

import (
	"log/slog"
	"time"

	"github.com/VTGare/boe-tea-go/router"
)

// Logging records every invocation: Info for success and user rejections
// (bad input, failed checks, cooldowns), Error for the rest.
// A nil logger discards everything.
func Logging(logger *slog.Logger) router.Middleware {
	if logger == nil {
		logger = slog.New(slog.DiscardHandler)
	}

	return func(next router.Handler) router.Handler {
		return func(ctx *router.Context) error {
			start := time.Now()
			err := next(ctx)

			attrs := []slog.Attr{
				slog.String("command", ctx.Command.QualifiedName()),
				slog.String("source", ctx.Source.String()),
				slog.String("user_id", ctx.AuthorID()),
				slog.String("guild_id", ctx.GuildID()),
				slog.String("channel_id", ctx.ChannelID()),
				slog.Duration("duration", time.Since(start)),
			}

			if u := ctx.Author(); u != nil {
				attrs = append(attrs, slog.String("user", u.Username))
			}

			level, msg := slog.LevelInfo, "command executed"
			if err != nil {
				if reason, ok := router.UserMessageOf(err); ok {
					msg = "command rejected"
					attrs = append(attrs, slog.String("reason", reason))
				} else {
					level, msg = slog.LevelError, "command failed"
					attrs = append(attrs, slog.Any("error", err))
				}
			}

			logger.LogAttrs(ctx.Context(), level, msg, attrs...)

			return err
		}
	}
}
