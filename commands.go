package pomomo

import (
	"github.com/bwmarrin/discordgo"
)

const (
	PomodoroOption   = "pomodoro"
	ShortBreakOption = "short_break"
	LongBreakOption  = "long_break"
	IntervalsOption  = "intervals"
	NoDeafenOption   = "no_deafen"
	NoMuteOption     = "no_mute"
)

func float64Ptr(f float64) *float64 {
	return &f
}

var StartCommand = discordgo.ApplicationCommand{
	Name:        "start",
	Description: "start pomodoro session",
	Options: []*discordgo.ApplicationCommandOption{
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        PomodoroOption,
			Description: "pomodoro duration in minutes (Default: 20)",
			MinValue:    float64Ptr(0),
			MaxValue:    240,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        ShortBreakOption,
			Description: "short break duration in minutes (Default: 5)",
			MinValue:    float64Ptr(0),
			MaxValue:    240,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        LongBreakOption,
			Description: "long break duration in minutes (Default: 15)",
			MinValue:    float64Ptr(0),
			MaxValue:    240,
		},
		{
			Type:        discordgo.ApplicationCommandOptionInteger,
			Name:        IntervalsOption,
			Description: "number of intervals between long breaks (Default: 4)",
			MinValue:    float64Ptr(1),
			MaxValue:    20,
		},
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        NoDeafenOption,
			Description: "participants will not be deafened during pomodoro intervals (Default: false)",
		},
		{
			Type:        discordgo.ApplicationCommandOptionBoolean,
			Name:        NoMuteOption,
			Description: "participants will not be muted during pomodoro intervals (Default: false)",
		},
	},
}
