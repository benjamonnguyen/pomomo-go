package main

import (
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/benjamonnguyen/pomomo-go/disgo"
	"github.com/disgoorg/disgo/discord"
)

type DiscordMessenger interface {
	EditChannelMessage(cID pomomo.TextChannelID, messageID string, components ...discord.LayoutComponent) (*discord.Message, error)
	Respond(it discord.Interaction, wait bool, components ...discord.LayoutComponent) (*discord.Message, error)
	EditResponse(it discord.Interaction, components ...discord.LayoutComponent) (*discord.Message, error)
	DeferMessageCreate(it discord.Interaction, ephemeral bool) (disgo.Followup, error)
	DeferMessageUpdate(it discord.Interaction) (disgo.Followup, error)
}

var _ DiscordMessenger = (*disgo.Messenger)(nil)

type InteractionID struct {
	Type    string
	TextCID pomomo.TextChannelID
}

func FromCustomID(customID string) (InteractionID, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 2 {
		return InteractionID{}, fmt.Errorf("invalid customID: %s", customID)
	}
	return InteractionID{
		Type:    parts[0],
		TextCID: pomomo.TextChannelID(parts[1]),
	}, nil
}

func (id InteractionID) ToCustomID() string {
	return fmt.Sprintf("%s:%s", id.Type, id.TextCID)
}

type Color int

const (
	ColorDefault           Color = 0x000000
	ColorWhite             Color = 0xffffff
	ColorAqua              Color = 0x1abc9c
	ColorGreen             Color = 0x57f287
	ColorBlue              Color = 0x3498db
	ColorYellow            Color = 0xfee75c
	ColorPurple            Color = 0x9b59b6
	ColorLuminousVividPink Color = 0xe91e63
	ColorFuchsia           Color = 0xeb459e
	ColorGold              Color = 0xf1c40f
	ColorOrange            Color = 0xe67e22
	ColorRed               Color = 0xed4245
	ColorGrey              Color = 0x95a5a6
	ColorNavy              Color = 0x34495e
	ColorDarkAqua          Color = 0x11806a
	ColorDarkGreen         Color = 0x1f8b4c
	ColorDarkBlue          Color = 0x206694
	ColorDarkPurple        Color = 0x71368a
	ColorDarkVividPink     Color = 0xad1457
	ColorDarkGold          Color = 0xc27c0e
	ColorDarkOrange        Color = 0xa84300
	ColorDarkRed           Color = 0x992d22
	ColorDarkGrey          Color = 0x979c9f
	ColorDarkerGrey        Color = 0x7f8c8d
	ColorLightGrey         Color = 0xbcc0c0
	ColorDarkNavy          Color = 0x2c3e50
	ColorBlurple           Color = 0x5865f2
	ColorGreyple           Color = 0x99aab5
	ColorDarkButNotBlack   Color = 0x2c2f33
	ColorNotQuiteBlack     Color = 0x23272a
)

func TextDisplay(content string) discord.TextDisplayComponent {
	return discord.TextDisplayComponent{
		Content: content,
	}
}

const (
	timerBarFilledChar = "⣶"
	timerBarEmptyChar  = "⡀"
)

func SessionMessageComponents(s models.Session) []discord.LayoutComponent {
	if s.Record.Status == pomomo.SessionEnded {
		return []discord.LayoutComponent{
			TextDisplay(getFarewell()),
			TextDisplay(fmt.Sprintf("**Session complete** · %s · %d 🍅", formatDuration(time.Since(s.StartedAt)), s.Record.CompletedPomodoros)),
		}
	}
	// action row

	actionRow := discord.ActionRowComponent{
		Components: []discord.InteractiveComponent{
			discord.ButtonComponent{
				Label: "Join",
				Style: discord.ButtonStylePrimary,
				CustomID: InteractionID{
					Type:    "join",
					TextCID: s.Record.TextCID,
				}.ToCustomID(),
			},
			discord.ButtonComponent{
				Label: "End",
				Style: discord.ButtonStyleDanger,
				CustomID: InteractionID{
					Type:    "end",
					TextCID: s.Record.TextCID,
				}.ToCustomID(),
			},
			discord.ButtonComponent{
				Label: "Skip",
				Style: discord.ButtonStyleSecondary,
				CustomID: InteractionID{
					Type:    "skip",
					TextCID: s.Record.TextCID,
				}.ToCustomID(),
			},
		},
	}

	// settings
	settingsTextParts := []string{
		"### Session Settings",
		fmt.Sprintf("%s: %d min", pomomo.PomodoroInterval, int(s.Settings.Pomodoro.Minutes())),
		fmt.Sprintf("%s: %d min", pomomo.ShortBreakInterval, int(s.Settings.ShortBreak.Minutes())),
		fmt.Sprintf("%s: %d min", pomomo.LongBreakInterval, int(s.Settings.LongBreak.Minutes())),
		fmt.Sprintf("%s: %d | %d", "Interval", s.PomodoroInCycle(), s.Settings.Intervals),
	}
	switch s.Record.CurrentInterval {
	case pomomo.PomodoroInterval:
		settingsTextParts[1] = fmt.Sprintf("**%s**\n%s", settingsTextParts[1], timerBar(s))
	case pomomo.ShortBreakInterval:
		settingsTextParts[2] = fmt.Sprintf("**%s**\n%s", settingsTextParts[2], timerBar(s))
	case pomomo.LongBreakInterval:
		settingsTextParts[3] = fmt.Sprintf("**%s**\n%s", settingsTextParts[3], timerBar(s))
	default:
		settingsTextParts = append(settingsTextParts, timerBar(s))
	}
	accentColor := ColorGreen
	if s.Record.Status == pomomo.SessionPaused {
		accentColor = ColorLightGrey
	}
	settingsContainer := discord.ContainerComponent{
		Components: []discord.ContainerSubComponent{
			discord.TextDisplayComponent{
				Content: strings.Join(settingsTextParts, "\n"),
			},
		},
		AccentColor: int(accentColor),
	}

	//
	var components []discord.LayoutComponent
	if s.Greeting != "" {
		components = append(components, TextDisplay(s.Greeting))
	}
	components = append(components, settingsContainer, actionRow)
	return components
}

func timerBar(s models.Session) string {
	const length = 20
	filledChar := timerBarFilledChar
	emptyChar := timerBarEmptyChar
	remaining := s.TimeRemaining().Minutes()
	if remaining <= 0 {
		return strings.Repeat(emptyChar, length)
	}
	percentage := remaining / s.CurrentDuration().Minutes()
	filled := min(int(math.Round(percentage*length*10)/10), length)
	return strings.Repeat(filledChar, filled) + strings.Repeat(emptyChar, length-filled)
}

var greetings = []string{
	"Howdy howdy! Let's do this thang :cowboy:",
	"Hey there! Let's get started :books:",
	"It's productivity o'clock! :alarm_clock:",
	"Let's ketchup on some work! :tomato:",
}

var farewells = []string{
	"See you later! 👋",
	"Goodbye! 👋",
	"Great work! 👋",
}

func getGreeting() string {
	return greetings[rand.Intn(len(greetings))]
}

func getFarewell() string {
	return farewells[rand.Intn(len(farewells))]
}
