package disgo

import (
	"github.com/benjamonnguyen/pomomo-go"
	"github.com/disgoorg/disgo/discord"
	"github.com/disgoorg/disgo/rest"
)

// Followup sends the deferred response's content.
type Followup func(components ...discord.LayoutComponent) (*discord.Message, error)

// Messenger sends Components V2 messages and interaction responses.
type Messenger struct {
	rest rest.Rest
}

func NewMessenger(r rest.Rest) *Messenger {
	return &Messenger{
		rest: r,
	}
}

func (m *Messenger) EditChannelMessage(cID pomomo.TextChannelID, messageID string, components ...discord.LayoutComponent) (*discord.Message, error) {
	ids, err := ParseIDs(string(cID), messageID)
	if err != nil {
		return nil, err
	}
	flags := discord.MessageFlagIsComponentsV2
	return m.rest.UpdateMessage(ids[0], ids[1], discord.MessageUpdate{
		Flags:      &flags,
		Components: &components,
	})
}

// Respond returns message only when wait == true
func (m *Messenger) Respond(it discord.Interaction, wait bool, components ...discord.LayoutComponent) (*discord.Message, error) {
	if err := m.rest.CreateInteractionResponse(it.ID(), it.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeCreateMessage,
		Data: discord.MessageCreate{
			Flags:      discord.MessageFlagIsComponentsV2,
			Components: components,
		},
	}); err != nil {
		return nil, err
	}
	if wait {
		return m.rest.GetInteractionResponse(it.ApplicationID(), it.Token())
	}
	return nil, nil
}

func (m *Messenger) EditResponse(it discord.Interaction, components ...discord.LayoutComponent) (*discord.Message, error) {
	return m.rest.UpdateInteractionResponse(it.ApplicationID(), it.Token(), discord.MessageUpdate{
		Components: &components,
	})
}

func (m *Messenger) DeferMessageCreate(it discord.Interaction, ephemeral bool) (Followup, error) {
	var flags discord.MessageFlags
	if ephemeral {
		flags = flags.Add(discord.MessageFlagEphemeral)
	}
	if err := m.rest.CreateInteractionResponse(it.ID(), it.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeDeferredCreateMessage,
		Data: discord.MessageCreate{
			Flags: flags,
		},
	}); err != nil {
		return nil, err
	}
	return func(components ...discord.LayoutComponent) (*discord.Message, error) {
		return m.rest.CreateFollowupMessage(it.ApplicationID(), it.Token(), discord.MessageCreate{
			Flags:      discord.MessageFlagIsComponentsV2,
			Components: components,
		})
	}, nil
}

// FollowupWithFiles sends a deferred response's content with files attached.
// Components V2 only show files that a component references as attachment://<name>.
func (m *Messenger) FollowupWithFiles(it discord.Interaction, files []*discord.File, components ...discord.LayoutComponent) (*discord.Message, error) {
	return m.rest.CreateFollowupMessage(it.ApplicationID(), it.Token(), discord.MessageCreate{
		Flags:      discord.MessageFlagIsComponentsV2,
		Components: components,
		Files:      files,
	})
}

// DeferMessageUpdate acks a component interaction; the followup edits the message the component is on.
func (m *Messenger) DeferMessageUpdate(it discord.Interaction) (Followup, error) {
	if err := m.rest.CreateInteractionResponse(it.ID(), it.Token(), discord.InteractionResponse{
		Type: discord.InteractionResponseTypeDeferredUpdateMessage,
	}); err != nil {
		return nil, err
	}
	return func(components ...discord.LayoutComponent) (*discord.Message, error) {
		return m.rest.UpdateInteractionResponse(it.ApplicationID(), it.Token(), discord.MessageUpdate{
			Components: &components,
		})
	}, nil
}
