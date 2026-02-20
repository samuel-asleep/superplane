package telegram

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"

	"github.com/mitchellh/mapstructure"
	"github.com/superplanehq/superplane/pkg/configuration"
	"github.com/superplanehq/superplane/pkg/core"
	"github.com/superplanehq/superplane/pkg/registry"
)

func init() {
	registry.RegisterIntegration("telegram", &Telegram{})
}

type Telegram struct{}

type Configuration struct {
	BotToken      string `json:"botToken" mapstructure:"botToken"`
	DefaultChatID string `json:"defaultChatId" mapstructure:"defaultChatId"`
}

type Metadata struct {
	BotID    int    `json:"botId" mapstructure:"botId"`
	Username string `json:"username" mapstructure:"username"`
}

// SubscriptionConfiguration holds event types a subscription listens for.
type SubscriptionConfiguration struct {
	EventTypes []string `json:"eventTypes" mapstructure:"eventTypes"`
}

func (t *Telegram) Name() string {
	return "telegram"
}

func (t *Telegram) Label() string {
	return "Telegram"
}

func (t *Telegram) Icon() string {
	return "telegram"
}

func (t *Telegram) Description() string {
	return "Send messages and react to mentions via Telegram Bot API"
}

func (t *Telegram) Instructions() string {
	return `
Create a bot via **@BotFather** on Telegram and paste the **Bot Token** below.
`
}

func (t *Telegram) Configuration() []configuration.Field {
	return []configuration.Field{
		{
			Name:        "botToken",
			Label:       "Bot Token",
			Type:        configuration.FieldTypeString,
			Required:    true,
			Sensitive:   true,
			Description: "Telegram bot token from BotFather",
		},
		{
			Name:        "defaultChatId",
			Label:       "Default Chat ID",
			Type:        configuration.FieldTypeString,
			Required:    false,
			Description: "Default Telegram chat ID used when the Send Message component does not specify one",
		},
	}
}

func (t *Telegram) Components() []core.Component {
	return []core.Component{
		&SendTextMessage{},
	}
}

func (t *Telegram) Triggers() []core.Trigger {
	return []core.Trigger{
		&OnMention{},
	}
}

func (t *Telegram) Sync(ctx core.SyncContext) error {
	botTokenBytes, err := ctx.Integration.GetConfig("botToken")
	if err != nil {
		return fmt.Errorf("botToken is required")
	}

	botToken := string(botTokenBytes)
	if botToken == "" {
		return fmt.Errorf("botToken is required")
	}

	client, err := NewClient(ctx.Integration)
	if err != nil {
		return err
	}

	me, err := client.GetMe()
	if err != nil {
		return fmt.Errorf("failed to verify bot token: %v", err)
	}

	ctx.Integration.SetMetadata(Metadata{
		BotID:    me.ID,
		Username: me.Username,
	})

	// Register the webhook URL with Telegram so updates are pushed to us.
	baseURL := ctx.WebhooksBaseURL
	if baseURL == "" {
		baseURL = ctx.BaseURL
	}

	if baseURL != "" {
		webhookURL := fmt.Sprintf("%s/api/v1/integrations/%s/events", baseURL, ctx.Integration.ID())
		if err := client.SetWebhook(webhookURL); err != nil {
			return fmt.Errorf("failed to register webhook: %v", err)
		}
	}

	ctx.Integration.Ready()
	return nil
}

func (t *Telegram) HandleRequest(ctx core.HTTPRequestContext) {
	body, err := io.ReadAll(ctx.Request.Body)
	if err != nil {
		ctx.Logger.Errorf("telegram: failed to read request body: %v", err)
		ctx.Response.WriteHeader(http.StatusBadRequest)
		return
	}

	var update TelegramUpdate
	if err := json.Unmarshal(body, &update); err != nil {
		ctx.Logger.Errorf("telegram: failed to parse update: %v", err)
		ctx.Response.WriteHeader(http.StatusBadRequest)
		return
	}

	if update.Message == nil {
		ctx.Response.WriteHeader(http.StatusOK)
		return
	}

	// Only dispatch messages where the bot is mentioned.
	metadata := Metadata{}
	if err := mapstructure.Decode(ctx.Integration.GetMetadata(), &metadata); err != nil {
		ctx.Logger.Errorf("telegram: failed to decode metadata: %v", err)
		ctx.Response.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !isBotMentioned(update.Message, metadata) {
		ctx.Response.WriteHeader(http.StatusOK)
		return
	}

	subscriptions, err := ctx.Integration.ListSubscriptions()
	if err != nil {
		ctx.Logger.Errorf("telegram: failed to list subscriptions: %v", err)
		ctx.Response.WriteHeader(http.StatusInternalServerError)
		return
	}

	for _, sub := range subscriptions {
		c := SubscriptionConfiguration{}
		if err := mapstructure.Decode(sub.Configuration(), &c); err != nil {
			ctx.Logger.Errorf("telegram: failed to decode subscription config: %v", err)
			continue
		}

		if !slices.Contains(c.EventTypes, "mention") {
			continue
		}

		if err := sub.SendMessage(update.Message); err != nil {
			ctx.Logger.Errorf("telegram: failed to dispatch message: %v", err)
		}
	}

	ctx.Response.WriteHeader(http.StatusOK)
}

// isBotMentioned checks if the bot is @mentioned in the message.
// It checks for "mention" entities (where the text contains @username)
// and "text_mention" entities (where the user is referenced without a username).
func isBotMentioned(msg *TelegramMessage, metadata Metadata) bool {
	for _, entity := range msg.Entities {
		switch entity.Type {
		case "mention":
			// Extract the @username from the message text.
			if metadata.Username != "" && entity.Offset+entity.Length <= len(msg.Text) {
				mentioned := msg.Text[entity.Offset : entity.Offset+entity.Length]
				if mentioned == "@"+metadata.Username {
					return true
				}
			}
		case "text_mention":
			// text_mention includes a User object for users without usernames.
			if entity.User != nil && entity.User.ID == metadata.BotID {
				return true
			}
		}
	}

	return false
}

func (t *Telegram) Cleanup(ctx core.IntegrationCleanupContext) error {
	return nil
}

func (t *Telegram) ListResources(resourceType string, ctx core.ListResourcesContext) ([]core.IntegrationResource, error) {
	return []core.IntegrationResource{}, nil
}

func (t *Telegram) Actions() []core.Action {
	return []core.Action{}
}

func (t *Telegram) HandleAction(ctx core.IntegrationActionContext) error {
	return nil
}
