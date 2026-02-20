package telegram

import (
	_ "embed"
	"sync"

	"github.com/superplanehq/superplane/pkg/utils"
)

//go:embed example_output_send_text_message.json
var exampleOutputSendTextMessageBytes []byte

//go:embed example_data_on_mention.json
var exampleDataOnMentionBytes []byte

var exampleOutputSendTextMessageOnce sync.Once
var exampleOutputSendTextMessage map[string]any

var exampleDataOnMentionOnce sync.Once
var exampleDataOnMention map[string]any

func (c *SendTextMessage) ExampleOutput() map[string]any {
	return utils.UnmarshalEmbeddedJSON(&exampleOutputSendTextMessageOnce, exampleOutputSendTextMessageBytes, &exampleOutputSendTextMessage)
}

func (t *OnMention) ExampleData() map[string]any {
	return utils.UnmarshalEmbeddedJSON(&exampleDataOnMentionOnce, exampleDataOnMentionBytes, &exampleDataOnMention)
}
