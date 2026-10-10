package service

import (
	"context"
	"log/slog"

	"github.com/rs/zerolog"
)

// slogBridge routes the oidc library's slog output into aio's zerolog logger
// so provider errors show up with the rest of the server logs.
type slogBridge struct {
	log   zerolog.Logger
	attrs []slog.Attr
}

func newSlogBridge(log zerolog.Logger) *slog.Logger {
	return slog.New(&slogBridge{log: log.With().Str("module", "oidc").Logger()})
}

func toZerologLevel(l slog.Level) zerolog.Level {
	switch {
	case l >= slog.LevelError:
		return zerolog.ErrorLevel
	case l >= slog.LevelWarn:
		return zerolog.WarnLevel
	case l >= slog.LevelInfo:
		return zerolog.InfoLevel
	default:
		return zerolog.DebugLevel
	}
}

func (b *slogBridge) Enabled(_ context.Context, l slog.Level) bool {
	lvl := toZerologLevel(l)
	return lvl >= b.log.GetLevel() && lvl >= zerolog.GlobalLevel()
}

func (b *slogBridge) Handle(_ context.Context, r slog.Record) error {
	ev := b.log.WithLevel(toZerologLevel(r.Level))
	for _, a := range b.attrs {
		ev = ev.Any(a.Key, a.Value.Any())
	}
	r.Attrs(func(a slog.Attr) bool {
		ev = ev.Any(a.Key, a.Value.Any())
		return true
	})
	ev.Msg(r.Message)
	return nil
}

func (b *slogBridge) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &slogBridge{log: b.log, attrs: append(append([]slog.Attr{}, b.attrs...), attrs...)}
}

func (b *slogBridge) WithGroup(string) slog.Handler { return b }
