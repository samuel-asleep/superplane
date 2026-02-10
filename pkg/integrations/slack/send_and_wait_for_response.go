package slack

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/superplanehq/superplane/pkg/configuration"
	"github.com/superplanehq/superplane/pkg/core"
)

const (
	ChannelReceived = "received"
	ChannelTimeout  = "timeout"
)

type SendAndWaitForResponse struct{}

type SendAndWaitForResponseConfiguration struct {
	Channel string   `json:"channel" mapstructure:"channel"`
	Message string   `json:"message" mapstructure:"message"`
	Timeout *int     `json:"timeout,omitempty" mapstructure:"timeout,omitempty"`
	Buttons []Button `json:"buttons" mapstructure:"buttons"`
}

type Button struct {
	Name  string `json:"name" mapstructure:"name"`
	Value string `json:"value" mapstructure:"value"`
}

type SendAndWaitForResponseMetadata struct {
	Channel           *ChannelMetadata `json:"channel,omitempty" mapstructure:"channel,omitempty"`
	MessageTS         string           `json:"messageTs,omitempty" mapstructure:"messageTs,omitempty"`
	ResponseTS        string           `json:"responseTs,omitempty" mapstructure:"responseTs,omitempty"`
	SelectedValue     string           `json:"selectedValue,omitempty" mapstructure:"selectedValue,omitempty"`
	State             string           `json:"state" mapstructure:"state"`
	SubscriptionID    *string          `json:"subscriptionId,omitempty" mapstructure:"subscriptionId,omitempty"`
}

func (c *SendAndWaitForResponse) Name() string {
	return "slack.sendAndWaitForResponse"
}

func (c *SendAndWaitForResponse) Label() string {
	return "Send and Wait for Response"
}

func (c *SendAndWaitForResponse) Description() string {
	return "Send a message with buttons to a Slack channel and wait for a response"
}

func (c *SendAndWaitForResponse) Documentation() string {
	return `The Send and Wait for Response component sends a message with interactive buttons to a Slack channel and waits for the user to click one of them.

## Use Cases

- **Approval workflows**: Request approval or input from a user before proceeding
- **Decision points**: Pause a workflow until a human selects an option
- **Interactive notifications**: Send notifications that require user acknowledgment or choice
- **Slash-command style flows**: Implement interactive flows with structured replies

## Configuration

- **Channel**: Select the Slack channel to send the message to
- **Message**: The message text to send (supports Slack formatting)
- **Timeout**: Optional maximum time to wait in seconds (if not set, waits indefinitely)
- **Buttons**: Set of 1-4 buttons, each with a name (label shown to user) and value (emitted when clicked)

## Output Channels

- **Received**: Emits when the user clicks a button; payload includes the selected button's value
- **Timeout**: Emits when no button click is received within the configured timeout

## Notes

- The Slack app must be installed and have permission to post to the selected channel
- Button clicks are processed through Slack's interactivity endpoint
- Only the first button click is captured; subsequent clicks are ignored
- If timeout is not configured, the component waits indefinitely until a button is clicked`
}

func (c *SendAndWaitForResponse) Icon() string {
	return "slack"
}

func (c *SendAndWaitForResponse) Color() string {
	return "gray"
}

func (c *SendAndWaitForResponse) ExampleOutput() map[string]any {
	return map[string]any{
		"value":      "approve",
		"messageTs":  "1234567890.123456",
		"responseTs": "1234567891.123456",
		"user": map[string]any{
			"id":   "U123ABC456",
			"name": "john.doe",
		},
	}
}

func (c *SendAndWaitForResponse) OutputChannels(configuration any) []core.OutputChannel {
	return []core.OutputChannel{
		{Name: ChannelReceived, Label: "Received", Description: "Emitted when a button is clicked"},
		{Name: ChannelTimeout, Label: "Timeout", Description: "Emitted when timeout is reached without a response"},
	}
}

func (c *SendAndWaitForResponse) Configuration() []configuration.Field {
	return []configuration.Field{
		{
			Name:     "channel",
			Label:    "Channel",
			Type:     configuration.FieldTypeIntegrationResource,
			Required: true,
			TypeOptions: &configuration.TypeOptions{
				Resource: &configuration.ResourceTypeOptions{
					Type: "channel",
				},
			},
		},
		{
			Name:     "message",
			Label:    "Message",
			Type:     configuration.FieldTypeText,
			Required: true,
		},
		{
			Name:        "timeout",
			Label:       "Timeout (seconds)",
			Type:        configuration.FieldTypeNumber,
			Required:    false,
			Description: "Maximum time to wait for a response (leave empty to wait indefinitely)",
		},
		{
			Name:        "buttons",
			Label:       "Buttons",
			Description: "Set of 1-4 buttons for the user to respond with",
			Type:        configuration.FieldTypeList,
			Required:    true,
			TypeOptions: &configuration.TypeOptions{
				List: &configuration.ListTypeOptions{
					ItemLabel: "Button",
					ItemDefinition: &configuration.ListItemDefinition{
						Type: configuration.FieldTypeObject,
						Schema: []configuration.Field{
							{
								Name:     "name",
								Label:    "Name",
								Type:     configuration.FieldTypeString,
								Required: true,
							},
							{
								Name:     "value",
								Label:    "Value",
								Type:     configuration.FieldTypeString,
								Required: true,
							},
						},
					},
				},
			},
		},
	}
}

func (c *SendAndWaitForResponse) Setup(ctx core.SetupContext) error {
	var config SendAndWaitForResponseConfiguration
	if err := mapstructure.Decode(ctx.Configuration, &config); err != nil {
		return fmt.Errorf("failed to decode configuration: %w", err)
	}

	if config.Channel == "" {
		return errors.New("channel is required")
	}

	if config.Message == "" {
		return errors.New("message is required")
	}

	if len(config.Buttons) == 0 {
		return errors.New("at least one button is required")
	}

	if len(config.Buttons) > 4 {
		return errors.New("maximum of 4 buttons allowed")
	}

	// Validate button names and values
	for i, button := range config.Buttons {
		if button.Name == "" {
			return fmt.Errorf("button %d: name is required", i+1)
		}
		if button.Value == "" {
			return fmt.Errorf("button %d: value is required", i+1)
		}
	}

	client, err := NewClient(ctx.Integration)
	if err != nil {
		return fmt.Errorf("failed to create Slack client: %w", err)
	}

	channelInfo, err := client.GetChannelInfo(config.Channel)
	if err != nil {
		return fmt.Errorf("channel validation failed: %w", err)
	}

	// Subscribe to integration messages (for button click interactions)
	subscriptionConfig := map[string]any{
		"componentName": c.Name(),
	}
	subscriptionID, err := ctx.Integration.Subscribe(subscriptionConfig)
	if err != nil {
		return fmt.Errorf("failed to subscribe to integration messages: %w", err)
	}

	subscriptionIDStr := subscriptionID.String()
	metadata := SendAndWaitForResponseMetadata{
		Channel: &ChannelMetadata{
			ID:   channelInfo.ID,
			Name: channelInfo.Name,
		},
		State:          "pending",
		SubscriptionID: &subscriptionIDStr,
	}

	return ctx.Metadata.Set(metadata)
}

func (c *SendAndWaitForResponse) ProcessQueueItem(ctx core.ProcessQueueContext) (*uuid.UUID, error) {
	return ctx.DefaultProcessing()
}

func (c *SendAndWaitForResponse) Execute(ctx core.ExecutionContext) error {
	var config SendAndWaitForResponseConfiguration
	if err := mapstructure.Decode(ctx.Configuration, &config); err != nil {
		return fmt.Errorf("failed to decode configuration: %w", err)
	}

	if config.Channel == "" {
		return errors.New("channel is required")
	}

	client, err := NewClient(ctx.Integration)
	if err != nil {
		return fmt.Errorf("failed to create Slack client: %w", err)
	}

	// Build interactive message with buttons
	blocks := c.buildMessageBlocks(config.Message, config.Buttons, ctx.ID.String())

	response, err := client.PostMessageWithBlocks(ChatPostMessageWithBlocksRequest{
		Channel: config.Channel,
		Text:    config.Message,
		Blocks:  blocks,
	})

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Store message timestamp for tracking
	var metadata SendAndWaitForResponseMetadata
	if err := mapstructure.Decode(ctx.Metadata.Get(), &metadata); err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}

	metadata.MessageTS = response.TS
	metadata.State = "waiting"
	if err := ctx.Metadata.Set(metadata); err != nil {
		return err
	}

	// Schedule timeout if configured
	if config.Timeout != nil && *config.Timeout > 0 {
		interval := time.Duration(*config.Timeout) * time.Second
		return ctx.Requests.ScheduleActionCall("timeout", map[string]any{}, interval)
	}

	return nil
}

func (c *SendAndWaitForResponse) buildMessageBlocks(message string, buttons []Button, executionID string) []any {
	blocks := []any{
		map[string]any{
			"type": "section",
			"text": map[string]string{
				"type": "mrkdwn",
				"text": message,
			},
		},
	}

	// Build button actions
	elements := []map[string]any{}
	for _, button := range buttons {
		elements = append(elements, map[string]any{
			"type":      "button",
			"text":      map[string]string{"type": "plain_text", "text": button.Name},
			"action_id": fmt.Sprintf("button_%s", button.Value),
			"value":     button.Value,
		})
	}

	blocks = append(blocks, map[string]any{
		"type":     "actions",
		"block_id": fmt.Sprintf("buttons_%s", executionID),
		"elements": elements,
	})

	return blocks
}

func (c *SendAndWaitForResponse) HandleWebhook(ctx core.WebhookRequestContext) (int, error) {
	return 200, nil
}

func (c *SendAndWaitForResponse) Actions() []core.Action {
	return []core.Action{
		{
			Name:        "timeout",
			Description: "Handle timeout when no response is received",
		},
		{
			Name:        "buttonClicked",
			Description: "Handle button click from Slack",
		},
	}
}

func (c *SendAndWaitForResponse) HandleAction(ctx core.ActionContext) error {
	switch ctx.Name {
	case "timeout":
		return c.handleTimeout(ctx)
	case "buttonClicked":
		return c.handleButtonClicked(ctx)
	default:
		return fmt.Errorf("unknown action: %s", ctx.Name)
	}
}

func (c *SendAndWaitForResponse) handleTimeout(ctx core.ActionContext) error {
	// Check if already responded
	var metadata SendAndWaitForResponseMetadata
	if err := mapstructure.Decode(ctx.Metadata.Get(), &metadata); err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}

	if metadata.State != "waiting" {
		// Already responded, don't emit timeout
		return nil
	}

	metadata.State = "timeout"
	if err := ctx.Metadata.Set(metadata); err != nil {
		return err
	}

	return ctx.ExecutionState.Emit(
		ChannelTimeout,
		"slack.response.timeout",
		[]any{map[string]any{
			"messageTs": metadata.MessageTS,
		}},
	)
}

func (c *SendAndWaitForResponse) handleButtonClicked(ctx core.ActionContext) error {
	// Check if already responded
	var metadata SendAndWaitForResponseMetadata
	if err := mapstructure.Decode(ctx.Metadata.Get(), &metadata); err != nil {
		return fmt.Errorf("failed to decode metadata: %w", err)
	}

	if metadata.State != "waiting" {
		// Already responded, ignore
		return nil
	}

	// Extract button click data from parameters
	selectedValue, _ := ctx.Parameters["value"].(string)
	responseTS, _ := ctx.Parameters["responseTs"].(string)
	user, _ := ctx.Parameters["user"].(map[string]any)

	metadata.SelectedValue = selectedValue
	metadata.ResponseTS = responseTS
	metadata.State = "responded"

	if err := ctx.Metadata.Set(metadata); err != nil {
		return err
	}

	return ctx.ExecutionState.Emit(
		ChannelReceived,
		"slack.response.received",
		[]any{map[string]any{
			"value":      selectedValue,
			"messageTs":  metadata.MessageTS,
			"responseTs": responseTS,
			"user":       user,
		}},
	)
}

func (c *SendAndWaitForResponse) Cancel(ctx core.ExecutionContext) error {
	return nil
}

func (c *SendAndWaitForResponse) Cleanup(ctx core.SetupContext) error {
	return nil
}

