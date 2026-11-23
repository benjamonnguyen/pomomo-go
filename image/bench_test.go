package image

import (
	"testing"
	"time"
)

func BenchmarkRenderRecap(b *testing.B) {
	monday := time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)
	r := Recap{WeekStart: monday, Now: monday.AddDate(0, 0, 3), Streak: 1, Days: [7]time.Duration{time.Hour, 2 * time.Hour}}
	for b.Loop() {
		if _, err := RenderRecap(r); err != nil {
			b.Fatal(err)
		}
	}
}
