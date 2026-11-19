package disgo

import (
	"log/slog"

	"github.com/charmbracelet/log"
)

// NewLogger returns a slog.Logger for disgo that writes through l.
// charmbracelet/log levels share slog's numeric values, so levels map 1:1.
// Call after l's output/level are configured; the prefix copy snapshots them.
func NewLogger(l *log.Logger) *slog.Logger {
	return slog.New(l.WithPrefix("disgo"))
}
