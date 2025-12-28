// Package dgutils contains utility wrappers around github.com/bwmarrin/discordgo
package dgutils

import (
	"fmt"
	"strings"

	"github.com/bwmarrin/discordgo"
)

// TODO responder retries
type deferredResponder struct {
	s  *discordgo.Session
	it *discordgo.Interaction
}

func NewDeferredResponder(s *discordgo.Session, it *discordgo.Interaction) *deferredResponder {
	return &deferredResponder{
		s:  s,
		it: it,
	}
}

type followup func(components ...discordgo.MessageComponent) (*discordgo.Message, error)

func (r *deferredResponder) DeferMessageCreate() (followup, error) {
	if err := r.s.InteractionRespond(r.it, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	}); err != nil {
		return nil, err
	}
	return func(components ...discordgo.MessageComponent) (*discordgo.Message, error) {
		return r.s.FollowupMessageCreate(r.it, true, &discordgo.WebhookParams{
			Components: components,
			Flags:      discordgo.MessageFlagsIsComponentsV2,
		})
	}, nil
}

func (r *deferredResponder) DeferMessageUpdate() (followup, error) {
	if err := r.s.InteractionRespond(r.it, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredMessageUpdate,
	}); err != nil {
		return nil, err
	}
	return func(components ...discordgo.MessageComponent) (*discordgo.Message, error) {
		return r.s.FollowupMessageEdit(r.it, r.it.Message.ID, &discordgo.WebhookEdit{
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
	Type, GuildID, ChannelID string
}

func FromCustomID(customID string) (InteractionID, error) {
	parts := strings.Split(customID, ":")
	if len(parts) != 3 {
		return InteractionID{}, fmt.Errorf("invalid customID: %s", customID)
	}
	return InteractionID{
		Type:      parts[0],
		GuildID:   parts[1],
		ChannelID: parts[2],
	}, nil
}

func (id InteractionID) ToCustomID() string {
	return fmt.Sprintf("%s:%s:%s", id.Type, id.GuildID, id.ChannelID)
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
