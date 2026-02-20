package telegram

import (
	"fmt"

	"github.com/mitchellh/mapstructure"
	"github.com/superplanehq/superplane/pkg/configuration"
	"github.com/superplanehq/superplane/pkg/core"
)

type OnMention struct{}

type OnMentionMetadata struct {
	SubscriptionID *string `json:"subscriptionId,omitempty" mapstructure:"subscriptionId,omitempty"`
}

func (t *OnMention) Name() string {
	return "telegram.onMention"
}

func (t *OnMention) Label() string {
	return "On Mention"
}

func (t *OnMention) Description() string {
	return "Fires when the bot is mentioned in a message"
}

func (t *OnMention) Documentation() string {
	return `The On Mention trigger fires when the Telegram bot is @mentioned in a message.

## Use Cases

- **Bot commands**: React to commands directed at the bot
- **Mentions**: Start a workflow when the bot is mentioned in a group

## Event Data

Each event includes the full Telegram message object:
- **message_id**: Unique message identifier
- **from**: Sender information (id, first_name, username)
- **chat**: Chat information (id, type, title)
- **text**: Message text
- **date**: Unix timestamp of the message
- **entities**: Special entities in the message (mentions, commands, etc.)

## Notes

- Only messages that @mention the bot will trigger this event
- Requires the Telegram integration to be configured with a valid bot token
- SuperPlane automatically registers a webhook with Telegram on integration setup`
}

func (t *OnMention) Icon() string {
	return "telegram"
}

func (t *OnMention) Color() string {
	return "blue"
}

func (t *OnMention) Configuration() []configuration.Field {
	return []configuration.Field{}
}

func (t *OnMention) Setup(ctx core.TriggerContext) error {
	var metadata OnMentionMetadata
	if err := mapstructure.Decode(ctx.Metadata.Get(), &metadata); err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}

	if metadata.SubscriptionID != nil {
		return nil
	}

	subscriptionID, err := ctx.Integration.Subscribe(SubscriptionConfiguration{
		EventTypes: []string{"mention"},
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to mentions: %w", err)
	}

	s := subscriptionID.String()
	return ctx.Metadata.Set(OnMentionMetadata{
		SubscriptionID: &s,
	})
}

func (t *OnMention) OnIntegrationMessage(ctx core.IntegrationMessageContext) error {
	return ctx.Events.Emit("telegram.mention", ctx.Message)
}

func (t *OnMention) HandleWebhook(ctx core.WebhookRequestContext) (int, error) {
	return 200, nil
}

func (t *OnMention) Actions() []core.Action {
	return []core.Action{}
}

func (t *OnMention) HandleAction(ctx core.TriggerActionContext) (map[string]any, error) {
	return nil, nil
}

func (t *OnMention) Cleanup(ctx core.TriggerContext) error {
	return nil
}
