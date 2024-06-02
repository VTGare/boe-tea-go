package logger

import (
	"github.com/getsentry/sentry-go"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func Sentry(dsn string) (zap.Option, error) {
	err := sentry.Init(sentry.ClientOptions{
		Dsn: dsn,
	})
	if err != nil {
		return nil, err
	}

	return zap.WrapCore(func(core zapcore.Core) zapcore.Core {
		return zapcore.RegisterHooks(core, func(entry zapcore.Entry) error {
			if entry.Level == zapcore.ErrorLevel ||
				entry.Level == zapcore.FatalLevel ||
				entry.Level == zapcore.WarnLevel {
				sentry.CaptureEvent(&sentry.Event{
					Timestamp: entry.Time,
					Logger:    entry.LoggerName,
					Message:   entry.Message,
					Extra: map[string]any{
						"Stack":  entry.Stack,
						"Caller": entry.Caller.String(),
					},
					Level: SentryLevel(entry.Level),
				})
			}

			return nil
		})
	}), nil
}

func SentryLevel(zapLevel zapcore.Level) sentry.Level {
	switch zapLevel {
	case zapcore.ErrorLevel:
		return sentry.LevelError
	case zapcore.WarnLevel:
		return sentry.LevelWarning
	case zapcore.FatalLevel:
		return sentry.LevelFatal
	}

	return sentry.LevelInfo
}
