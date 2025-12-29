package main

import (
	"strings"
	"testing"
	"time"
)

func TestSession_TimerBar(t *testing.T) {
	t.Parallel()

	const testDuration = 30 * time.Minute
	now := time.Now()

	baseSession := &Session{
		currentInterval: PomodoroInterval,
		settings: SessionSettings{
			pomodoro:   testDuration,
			shortBreak: 5 * time.Minute,
			longBreak:  15 * time.Minute,
			intervals:  4,
		},
	}

	testCases := []struct {
		name              string
		intervalStartTime time.Time // set to now - elapsed
		elapsed           time.Duration
		expectedFilled    int // number of filled characters (■)
		expectedEmpty     int // number of empty characters (□)
	}{
		{
			name:              "elapsed <= 0 (not started)",
			intervalStartTime: now.Add(1 * time.Minute), // future start
			elapsed:           -1 * time.Minute,
			expectedFilled:    20,
			expectedEmpty:     0,
		},
		{
			name:              "just started (elapsed = 0)",
			intervalStartTime: now,
			elapsed:           0,
			expectedFilled:    20,
			expectedEmpty:     0,
		},
		{
			name:              "half elapsed",
			intervalStartTime: now.Add(-15 * time.Minute), // 15 min elapsed
			elapsed:           15 * time.Minute,
			expectedFilled:    10,
			expectedEmpty:     10,
		},
		{
			name:              "full elapsed",
			intervalStartTime: now.Add(-30 * time.Minute), // 30 min elapsed
			elapsed:           30 * time.Minute,
			expectedFilled:    0,
			expectedEmpty:     20,
		},
		{
			name:              "overdue (elapsed > total)",
			intervalStartTime: now.Add(-35 * time.Minute), // 35 min elapsed
			elapsed:           35 * time.Minute,
			expectedFilled:    0,
			expectedEmpty:     20,
		},
		{
			name:              "quarter elapsed",
			intervalStartTime: now.Add(-7*time.Minute + -30*time.Second), // 7.5 min elapsed
			elapsed:           7*time.Minute + 30*time.Second,
			expectedFilled:    15, // remaining 22.5/30 = 0.75, 0.75*20 = 15
			expectedEmpty:     5,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			session := *baseSession
			session.intervalStartedAt = tc.intervalStartTime

			result := session.TimerBar()
			expected := strings.Repeat(timerBarFilledChar, tc.expectedFilled) + strings.Repeat(timerBarEmptyChar, tc.expectedEmpty)

			if result != expected {
				t.Errorf("TimerBar() = %q, want %q", result, expected)
			}
		})
	}
}
