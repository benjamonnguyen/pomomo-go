package pomomo

import (
	"github.com/disgoorg/disgo/discord"
)

const (
	PomodoroOption   = "pomodoro"
	ShortBreakOption = "short_break"
	LongBreakOption  = "long_break"
	IntervalsOption  = "intervals"
	NoDeafenOption   = "no_deafen"
	NoMuteOption     = "no_mute"
)

func intPtr(i int) *int {
	return &i
}

var StartCommand = discord.SlashCommandCreate{
	Name:        "start",
	Description: "start pomodoro session",
	Options: []discord.ApplicationCommandOption{
		discord.ApplicationCommandOptionInt{
			Name:        PomodoroOption,
			Description: "pomodoro duration in minutes (Default: 20)",
			MinValue:    intPtr(0),
			MaxValue:    intPtr(240),
		},
		discord.ApplicationCommandOptionInt{
			Name:        ShortBreakOption,
			Description: "short break duration in minutes (Default: 5)",
			MinValue:    intPtr(0),
			MaxValue:    intPtr(240),
		},
		discord.ApplicationCommandOptionInt{
			Name:        LongBreakOption,
			Description: "long break duration in minutes (Default: 15)",
			MinValue:    intPtr(0),
			MaxValue:    intPtr(240),
		},
		discord.ApplicationCommandOptionInt{
			Name:        IntervalsOption,
			Description: "number of intervals between long breaks (Default: 4)",
			MinValue:    intPtr(1),
			MaxValue:    intPtr(20),
		},
		discord.ApplicationCommandOptionBool{
			Name:        NoDeafenOption,
			Description: "participants will not be deafened during pomodoro intervals (Default: false)",
		},
		discord.ApplicationCommandOptionBool{
			Name:        NoMuteOption,
			Description: "participants will not be muted during pomodoro intervals (Default: false)",
		},
	},
}
