package image

import (
	"bytes"
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestRenderRecap(t *testing.T) {
	m := func(min int) time.Duration { return time.Duration(min) * time.Minute }
	monday := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	tests := map[string]Recap{
		"empty-monday": {Now: monday.Add(9 * time.Hour), Month: m(1310), Lifetime: m(4120)},
		"midweek-1fire": {Now: monday.AddDate(0, 0, 3).Add(15 * time.Hour), Streak: 1,
			Days: [7]time.Duration{m(50), m(100), m(25), m(75)}, Month: m(1560), Lifetime: m(4370)},
		"fullweek-2fire": {Now: monday.AddDate(0, 0, 6).Add(20 * time.Hour), Streak: 2,
			Days: [7]time.Duration{m(100), m(150), 0, m(225), m(50), m(25), m(175)}, Month: m(2950), Lifetime: m(9800)},
		"heavy-3fire": {Now: monday.AddDate(0, 0, 6).Add(22 * time.Hour), Streak: 3,
			Days: [7]time.Duration{m(400), m(525), m(475), m(600), m(350), m(250), m(450)}, Month: m(10890), Lifetime: m(72300)},
		"streak-out-of-range": {Now: monday.Add(12 * time.Hour), Streak: 9},
	}
	out := os.Getenv("POMOMO_RECAP_OUT") // set to write PNGs for review
	for name, r := range tests {
		r.WeekStart = monday
		b, err := RenderRecap(r)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		img, err := png.Decode(bytes.NewReader(b))
		if err != nil {
			t.Fatalf("%s: invalid png: %v", name, err)
		}
		if got := img.Bounds().Size(); got.X != recapW*scale || got.Y != recapH*scale {
			t.Errorf("%s: size %v", name, got)
		}
		if out != "" {
			if err := os.WriteFile(filepath.Join(out, name+".png"), b, 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestFormat(t *testing.T) {
	for d, want := range map[time.Duration]string{0: "0m", 45 * time.Minute: "45m", 2 * time.Hour: "2h", 160 * time.Minute: "2h 40m"} {
		if got := FormatDuration(d); got != want {
			t.Errorf("FormatDuration(%v) = %q, want %q", d, got, want)
		}
	}
	for d, want := range map[time.Duration]string{45 * time.Minute: "45m", 214*time.Hour + 50*time.Minute: "214h", 1205 * time.Hour: "1,205h"} {
		if got := FormatHours(d); got != want {
			t.Errorf("FormatHours(%v) = %q, want %q", d, got, want)
		}
	}
}
