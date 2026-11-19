package disgo

import (
	"bytes"
	"regexp"
	"strings"
	"testing"

	"github.com/charmbracelet/log"
)

func TestNewLogger(t *testing.T) {
	var buf bytes.Buffer
	l := log.NewWithOptions(&buf, log.Options{
		ReportTimestamp: true,
		ReportCaller:    true,
		Level:           log.InfoLevel,
	})
	sl := NewLogger(l)

	sl.Debug("hidden")
	sl.Warn("connection went zombie", "shard", 1)
	sl.Error("boom")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines (debug filtered), got %d:\n%s", len(lines), buf.String())
	}

	ts := regexp.MustCompile(`\d{4}/\d{2}/\d{2} \d{2}:\d{2}:\d{2}`)
	for _, line := range lines {
		if n := len(ts.FindAllString(line, -1)); n != 1 {
			t.Errorf("expected exactly one timestamp, got %d: %q", n, line)
		}
		if !strings.Contains(line, "disgo:") {
			t.Errorf("missing disgo prefix: %q", line)
		}
		if !strings.Contains(line, "logger_test.go") {
			t.Errorf("caller should be the slog call site: %q", line)
		}
	}
	if !strings.Contains(lines[0], "WARN") || !strings.Contains(lines[0], "shard=1") {
		t.Errorf("unexpected warn line: %q", lines[0])
	}
	if !strings.Contains(lines[1], "ERRO") {
		t.Errorf("unexpected error line: %q", lines[1])
	}
}
