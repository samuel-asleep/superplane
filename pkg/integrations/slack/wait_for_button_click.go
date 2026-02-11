package slack

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/superplanehq/superplane/pkg/configuration"
	"github.com/superplanehq/superplane/pkg/core"
)

type WaitForButtonClick struct{}

type WaitForButtonClickConfiguration struct {
	Channel string   `json:"channel" mapstructure:"channel"`
	Message string   `json:"message" mapstructure:"message"`
	Timeout *int     `json:"timeout" mapstructure:"timeout"`
	Buttons []Button `json:"buttons" mapstructure:"buttons"`
}

type Button struct {
	Name  string `json:"name" mapstructure:"name"`   // Label shown to user
	Value string `json:"value" mapstructure:"value"` // Value returned when clicked
}

type WaitForButtonClickMetadata struct {
	Channel        *ChannelMetadata `json:"channel" mapstructure:"channel"`
	MessageTS      string           `json:"message_ts" mapstructure:"message_ts"`
	ClickedValue   string           `json:"clicked_value" mapstructure:"clicked_value"`
	ClickedAt      string           `json:"clicked_at" mapstructure:"clicked_at"`
	TimedOut       bool             `json:"timed_out" mapstructure:"timed_out"`
	ButtonsClicked bool             `json:"buttons_clicked" mapstructure:"buttons_clicked"`
}

const (
	ChannelReceived = "received"
	ChannelTimeout  = "timeout"
)

func (c *WaitForButtonClick) Name() string {
	return "slack.waitForButtonClick"
}

func (c *WaitForButtonClick) Label() string {
	return "Wait for Button Click"
}

func (c *WaitForButtonClick) Description() string {
	return "Send a message with interactive buttons and wait for a user to click one"
}

func (c *WaitForButtonClick) Documentation() string {
	return `The Wait for Button Click component sends a message to a Slack channel or DM with interactive buttons and waits for the user to click one of the configured buttons.

## Use Cases

- **Request approval or input**: Get structured input from a user in Slack before applying or deploying (e.g., Approve / Reject buttons)
- **Pause a workflow**: Wait until a human selects an option (e.g., Confirm / Cancel)
- **Implement slash-command style flows**: Create interactive flows that need a structured reply via buttons

## Configuration

- **Channel**: Slack channel or DM channel name to post to (required)
- **Message**: Message text (supports Slack formatting, required)
- **Timeout**: Maximum time to wait in seconds (optional)
- **Buttons**: Set of 1–4 items, each with name (label) and value (required)

## Output Channels

- **Received**: Emits when the user clicks a button; payload includes the selected button's value
- **Timeout**: Emits when no button click is received within the configured timeout

## Behavior

- The message is posted with interactive buttons
- The workflow pauses until a button is clicked or timeout occurs
- Only the first button click is processed; subsequent clicks are ignored
- If timeout is not configured, the component waits indefinitely

## Notes

- The Slack app must be installed and have permission to post to the selected channel
- Supports Slack markdown formatting in message text
- Button clicks are processed through Slack's interactive components API`
}

func (c *WaitForButtonClick) Icon() string {
	return "slack"
}

func (c *WaitForButtonClick) Color() string {
	return "gray"
}

func (c *WaitForButtonClick) OutputChannels(configuration any) []core.OutputChannel {
	return []core.OutputChannel{
		{
			Name:        ChannelReceived,
			Label:       "Received",
			Description: "Emits when a button is clicked",
		},
		{
			Name:        ChannelTimeout,
			Label:       "Timeout",
			Description: "Emits when the timeout is reached without a button click",
		},
	}
}

func (c *WaitForButtonClick) Configuration() []configuration.Field {
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
			Name:     "timeout",
			Label:    "Timeout (seconds)",
			Type:     configuration.FieldTypeNumber,
			Required: false,
		},
		{
			Name:     "buttons",
			Label:    "Buttons",
			Type:     configuration.FieldTypeList,
			Required: true,
			TypeOptions: &configuration.TypeOptions{
				List: &configuration.ListTypeOptions{
					MinItems: 1,
					MaxItems: 4,
					Fields: []configuration.Field{
						{
							Name:     "name",
							Label:    "Button Label",
							Type:     configuration.FieldTypeString,
							Required: true,
						},
						{
							Name:     "value",
							Label:    "Button Value",
							Type:     configuration.FieldTypeString,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func (c *WaitForButtonClick) Setup(ctx core.SetupContext) error {
	var config WaitForButtonClickConfiguration
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
		return errors.New("maximum 4 buttons allowed")
	}

	// Validate button configuration
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

	metadata := WaitForButtonClickMetadata{
		Channel: &ChannelMetadata{
			ID:   channelInfo.ID,
			Name: channelInfo.Name,
		},
	}

	return ctx.Metadata.Set(metadata)
}

func (c *WaitForButtonClick) ProcessQueueItem(ctx core.ProcessQueueContext) (*uuid.UUID, error) {
	return ctx.DefaultProcessing()
}

func (c *WaitForButtonClick) Execute(ctx core.ExecutionContext) error {
	var config WaitForButtonClickConfiguration
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

	client, err := NewClient(ctx.Integration)
	if err != nil {
		return fmt.Errorf("failed to create Slack client: %w", err)
	}

	// Build Slack blocks with action buttons
	blocks := []interface{}{
		map[string]interface{}{
			"type": "section",
			"text": map[string]interface{}{
				"type": "mrkdwn",
				"text": config.Message,
			},
		},
		map[string]interface{}{
			"type": "actions",
			"elements": func() []interface{} {
				elements := []interface{}{}
				for _, button := range config.Buttons {
					elements = append(elements, map[string]interface{}{
						"type":      "button",
						"text":      map[string]interface{}{"type": "plain_text", "text": button.Name},
						"value":     button.Value,
						"action_id": fmt.Sprintf("button_%s", button.Value),
					})
				}
				return elements
			}(),
		},
	}

	// Post message to Slack
	response, err := client.PostMessage(ChatPostMessageRequest{
		Channel: config.Channel,
		Text:    config.Message, // Fallback text
		Blocks:  blocks,
	})

	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}

	// Store message timestamp for webhook lookup
	var metadata WaitForButtonClickMetadata
	if err := ctx.Metadata.Get(&metadata); err != nil {
		return fmt.Errorf("failed to get metadata: %w", err)
	}

	metadata.MessageTS = response.TS
	if err := ctx.Metadata.Set(metadata); err != nil {
		return fmt.Errorf("failed to set metadata: %w", err)
	}

	// Schedule timeout action if configured
	if config.Timeout != nil && *config.Timeout > 0 {
		if err := ctx.Requests.ScheduleActionCall("timeout", time.Duration(*config.Timeout)*time.Second, nil); err != nil {
			return fmt.Errorf("failed to schedule timeout: %w", err)
		}
	}

	return nil
}

func (c *WaitForButtonClick) HandleWebhook(ctx core.WebhookRequestContext) (int, error) {
	// Slack interactive payloads are sent as form-encoded data
	if err := ctx.Request.ParseForm(); err != nil {
		return 400, fmt.Errorf("failed to parse form: %w", err)
	}

	payloadStr := ctx.Request.FormValue("payload")
	if payloadStr == "" {
		return 400, errors.New("no payload found")
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(payloadStr), &payload); err != nil {
		return 400, fmt.Errorf("failed to parse payload: %w", err)
	}

	// Check if this is a block_actions interaction
	payloadType, ok := payload["type"].(string)
	if !ok || payloadType != "block_actions" {
		return 200, nil // Not our interaction type
	}

	// Extract message timestamp
	message, ok := payload["message"].(map[string]interface{})
	if !ok {
		return 400, errors.New("message not found in payload")
	}

	messageTS, ok := message["ts"].(string)
	if !ok {
		return 400, errors.New("message timestamp not found")
	}

	// Extract action value
	actions, ok := payload["actions"].([]interface{})
	if !ok || len(actions) == 0 {
		return 400, errors.New("no actions found in payload")
	}

	action, ok := actions[0].(map[string]interface{})
	if !ok {
		return 400, errors.New("invalid action format")
	}

	buttonValue, ok := action["value"].(string)
	if !ok {
		return 400, errors.New("button value not found")
	}

	// Find execution by message timestamp
	var metadata WaitForButtonClickMetadata
	if err := ctx.Metadata.Get(&metadata); err != nil {
		return 500, fmt.Errorf("failed to get metadata: %w", err)
	}

	// Check if this is the right message
	if metadata.MessageTS != messageTS {
		return 200, nil // Not our message
	}

	// Check if already processed
	if metadata.ButtonsClicked {
		return 200, nil // Already processed
	}

	// Process the button click
	params := map[string]interface{}{
		"value": buttonValue,
	}

	if err := ctx.Requests.InvokeAction("buttonClicked", params); err != nil {
		return 500, fmt.Errorf("failed to invoke action: %w", err)
	}

	return 200, nil
}

func (c *WaitForButtonClick) Actions() []core.Action {
	return []core.Action{
		{
			Name:           "buttonClicked",
			Description:    "Called when a button is clicked",
			UserAccessible: false,
			Parameters: []configuration.Field{
				{
					Name:     "value",
					Label:    "Button Value",
					Type:     configuration.FieldTypeString,
					Required: true,
				},
			},
		},
		{
			Name:           "timeout",
			Description:    "Called when the timeout is reached",
			UserAccessible: false,
			Parameters:     []configuration.Field{},
		},
	}
}

func (c *WaitForButtonClick) HandleAction(ctx core.ActionContext) error {
	var metadata WaitForButtonClickMetadata
	if err := ctx.Metadata.Get(&metadata); err != nil {
		return fmt.Errorf("failed to get metadata: %w", err)
	}

	switch ctx.Name {
	case "buttonClicked":
		// Check if already processed
		if metadata.ButtonsClicked {
			return nil
		}

		value, ok := ctx.Parameters["value"].(string)
		if !ok {
			return errors.New("button value not provided")
		}

		metadata.ClickedValue = value
		metadata.ClickedAt = time.Now().Format(time.RFC3339)
		metadata.ButtonsClicked = true

		if err := ctx.Metadata.Set(metadata); err != nil {
			return fmt.Errorf("failed to set metadata: %w", err)
		}

		return ctx.ExecutionState.Emit(
			ChannelReceived,
			"slack.button.clicked",
			[]any{map[string]interface{}{
				"value":      value,
				"clicked_at": metadata.ClickedAt,
			}},
		)

	case "timeout":
		// Check if already processed
		if metadata.ButtonsClicked {
			return nil // Button was clicked before timeout
		}

		metadata.TimedOut = true
		if err := ctx.Metadata.Set(metadata); err != nil {
			return fmt.Errorf("failed to set metadata: %w", err)
		}

		return ctx.ExecutionState.Emit(
			ChannelTimeout,
			"slack.button.timeout",
			[]any{},
		)

	default:
		return fmt.Errorf("unknown action: %s", ctx.Name)
	}
}

func (c *WaitForButtonClick) Cancel(ctx core.ExecutionContext) error {
	return nil
}

func (c *WaitForButtonClick) Cleanup(ctx core.SetupContext) error {
	return nil
}
