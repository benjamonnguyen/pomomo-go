// Package dgutils contains utility wrappers around github.com/bwmarrin/discordgo
package main

import (
	"fmt"
	"math"
	"strings"

	"github.com/benjamonnguyen/pomomo-go"
	"github.com/benjamonnguyen/pomomo-go/cmd/bot/models"
	"github.com/bwmarrin/discordgo"
)

type DiscordMessenger interface {
	EditChannelMessage(cID pomomo.TextChannelID, messageID string, components ...discordgo.MessageComponent) (*discordgo.Message, error)
	Respond(it *discordgo.Interaction, wait bool, components ...discordgo.MessageComponent) (*discordgo.Message, error)
	EditResponse(it *discordgo.Interaction, components ...discordgo.MessageComponent) (*discordgo.Message, error)
	DeferMessageCreate(it *discordgo.Interaction) (followup, error)
	DeferMessageUpdate(it *discordgo.Interaction) (followup, error)
}

func NewDiscordMessenger(client *discordgo.Session) DiscordMessenger {
	return &messenger{
		client: client,
	}
}

type messenger struct {
	client *discordgo.Session
}

func (m *messenger) EditChannelMessage(cID pomomo.TextChannelID, messageID string, components ...discordgo.MessageComponent) (*discordgo.Message, error) {
	return m.client.ChannelMessageEditComplex(&discordgo.MessageEdit{
		Channel:    string(cID),
		ID:         messageID,
		Flags:      discordgo.MessageFlagsIsComponentsV2,
		Components: &components,
	})
}

// Respond returns message only when wait == true
func (m *messenger) Respond(it *discordgo.Interaction, wait bool, components ...discordgo.MessageComponent) (*discordgo.Message, error) {
	if err := m.client.InteractionRespond(it, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags:      discordgo.MessageFlagsIsComponentsV2,
			Components: components,
		},
	}); err != nil {
		return nil, err
	}
	if wait {
		return m.client.InteractionResponse(it)
	}
	return nil, nil
}

func (m *messenger) EditResponse(it *discordgo.Interaction, components ...discordgo.MessageComponent) (*discordgo.Message, error) {
	return m.client.InteractionResponseEdit(it, &discordgo.WebhookEdit{
		Components: &components,
	})
}

type followup func(components ...discordgo.MessageComponent) (*discordgo.Message, error)

func (m *messenger) DeferMessageCreate(it *discordgo.Interaction) (followup, error) {
	if err := m.client.InteractionRespond(it, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		return nil, err
	}
	return func(components ...discordgo.MessageComponent) (*discordgo.Message, error) {
		return m.client.FollowupMessageCreate(it, true, &discordgo.WebhookParams{
			Components: components,
			Flags:      discordgo.MessageFlagsIsComponentsV2,
		})
	}, nil
}

func (m *messenger) DeferMessageUpdate(it *discordgo.Interaction) (followup, error) {
	if err := m.client.InteractionRespond(it, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}); err != nil {
		return nil, err
	}
	return func(components ...discordgo.MessageComponent) (*discordgo.Message, error) {
		return m.client.FollowupMessageEdit(it, it.Message.ID, &discordgo.WebhookEdit{
			Components: &components,
		})
	}, nil
}

func GetUser(m *discordgo.Interaction) *discordgo.User {
	if m.Member != nil {
		return m.Member.User
	}
	return m.User
}

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

func (c Color) ToInt() *int {
	i := int(c)
	return &i
}

func TextDisplay(content string) discordgo.TextDisplay {
	return discordgo.TextDisplay{
		Content: content,
	}
}

const (
	timerBarFilledChar = "⣶"
	timerBarEmptyChar  = "⡀"
)

func SessionMessageComponents(s models.Session) []discordgo.MessageComponent {
	if s.Record.Status == pomomo.SessionEnded {
		return []discordgo.MessageComponent{
			getEndMessage(),
		}
	}
	// action row
	skipButton := discordgo.Button{
		Label: "Skip",
		Style: discordgo.PrimaryButton,
		CustomID: InteractionID{
			Type:    "skip",
			TextCID: s.Record.TextCID,
		}.ToCustomID(),
	}
	endButton := discordgo.Button{
		Label: "End",
		Style: discordgo.DangerButton,
		CustomID: InteractionID{
			Type:    "end",
			TextCID: s.Record.TextCID,
		}.ToCustomID(),
	}

	actionRow := discordgo.ActionsRow{
		Components: []discordgo.MessageComponent{skipButton, endButton},
	}

	// settings
	settingsTextParts := []string{
		"### Session Settings",
		fmt.Sprintf("%s: %d min", pomomo.PomodoroInterval, int(s.Settings.Pomodoro.Minutes())),
		fmt.Sprintf("%s: %d min", pomomo.ShortBreakInterval, int(s.Settings.ShortBreak.Minutes())),
		fmt.Sprintf("%s: %d min", pomomo.LongBreakInterval, int(s.Settings.LongBreak.Minutes())),
		fmt.Sprintf("%s: %d | %d", "Interval", s.Stats.CompletedPomodoros%s.Settings.Intervals, s.Settings.Intervals),
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
	settingsContainer := discordgo.Container{
		Components: []discordgo.MessageComponent{
			discordgo.TextDisplay{
				Content: strings.Join(settingsTextParts, "\n"),
			},
		},
		AccentColor: accentColor.ToInt(),
	}

	//
	return []discordgo.MessageComponent{
		getStartMessage(),
		settingsContainer,
		actionRow,
	}
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

func getStartMessage() discordgo.MessageComponent {
	return TextDisplay("It's productivity o'clock!")
}

func getEndMessage() discordgo.MessageComponent {
	// TODO display stats in end message
	return TextDisplay("Good stuff!")
}
