// Package dgutils contains utility wrappers around github.com/bwmarrin/discordgo
package dgutils

import (
	"github.com/bwmarrin/discordgo"
)

// TODO responder retries
type interactionResponder struct {
	s  *discordgo.Session
	it *discordgo.Interaction
}

func NewInteractionResponder(s *discordgo.Session, it *discordgo.Interaction) *interactionResponder {
	return &interactionResponder{
		s:  s,
		it: it,
	}
}

func (r interactionResponder) DeferResponse() error {
	return r.s.InteractionRespond(r.it, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
	})
}

func (r interactionResponder) Followup(p discordgo.WebhookParams) (*discordgo.Message, error) {
	return r.s.FollowupMessageCreate(r.it, false, &p)
}

func GetUser(m *discordgo.Interaction) *discordgo.User {
	if m.Member != nil {
		return m.Member.User
	}
	return m.User
}
