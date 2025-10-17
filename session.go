package pomomo

import (
	"time"

	"github.com/google/uuid"
)

type Session struct {
	Id      uuid.UUID
	GuildId string

	//
	Pomodoro   time.Duration
	ShortBreak time.Duration
	LongBreak  time.Duration
	Intervals  int

	//
	CreatedAt time.Time
	UpdatedAt time.Time
	IsActive  bool
}
