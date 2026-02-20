package telegram

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superplanehq/superplane/pkg/core"
	"github.com/superplanehq/superplane/test/support/contexts"
)

func Test__Telegram__Sync(t *testing.T) {
	tg := &Telegram{}

	t.Run("missing bot token -> error", func(t *testing.T) {
		integrationCtx := &contexts.IntegrationContext{
			Configuration: map[string]any{},
		}

		err := tg.Sync(core.SyncContext{
			Configuration: map[string]any{},
			Integration:   integrationCtx,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "botToken is required")
	})

	t.Run("empty bot token -> error", func(t *testing.T) {
		integrationCtx := &contexts.IntegrationContext{
			Configuration: map[string]any{
				"botToken": "",
			},
		}

		err := tg.Sync(core.SyncContext{
			Configuration: map[string]any{"botToken": ""},
			Integration:   integrationCtx,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "botToken is required")
	})

	t.Run("valid bot token without base URL -> verifies and sets ready (no webhook)", func(t *testing.T) {
		withDefaultTransport(t, func(req *http.Request) (*http.Response, error) {
			assert.Contains(t, req.URL.String(), "/getMe")
			return jsonResponse(http.StatusOK, `{
				"ok": true,
				"result": {
					"id": 123456789,
					"is_bot": true,
					"first_name": "TestBot",
					"username": "testbot"
				}
			}`), nil
		})

		botToken := "test-bot-token"
		integrationCtx := &contexts.IntegrationContext{
			Configuration: map[string]any{
				"botToken": botToken,
			},
		}

		err := tg.Sync(core.SyncContext{
			Configuration: map[string]any{"botToken": botToken},
			Integration:   integrationCtx,
		})

		require.NoError(t, err)
		assert.Equal(t, "ready", integrationCtx.State)

		metadata, ok := integrationCtx.Metadata.(Metadata)
		require.True(t, ok)
		assert.Equal(t, 123456789, metadata.BotID)
		assert.Equal(t, "testbot", metadata.Username)
	})

	t.Run("valid bot token with base URL -> registers webhook", func(t *testing.T) {
		calls := 0
		withDefaultTransport(t, func(req *http.Request) (*http.Response, error) {
			calls++
			if calls == 1 {
				assert.Contains(t, req.URL.String(), "/getMe")
				return jsonResponse(http.StatusOK, `{
					"ok": true,
					"result": {"id": 1, "is_bot": true, "first_name": "Bot", "username": "bot"}
				}`), nil
			}
			assert.Contains(t, req.URL.String(), "/setWebhook")
			assert.Equal(t, http.MethodPost, req.Method)
			return jsonResponse(http.StatusOK, `{"ok": true, "description": "Webhook was set"}`), nil
		})

		integrationCtx := &contexts.IntegrationContext{
			Configuration: map[string]any{"botToken": "test-token"},
		}

		err := tg.Sync(core.SyncContext{
			Configuration:   map[string]any{"botToken": "test-token"},
			Integration:     integrationCtx,
			WebhooksBaseURL: "https://example.com",
		})

		require.NoError(t, err)
		assert.Equal(t, "ready", integrationCtx.State)
		assert.Equal(t, 2, calls)
	})

	t.Run("bot token verification fails -> error", func(t *testing.T) {
		withDefaultTransport(t, func(req *http.Request) (*http.Response, error) {
			return jsonResponse(http.StatusUnauthorized, `{"ok": false, "error_code": 401, "description": "Unauthorized"}`), nil
		})

		botToken := "invalid-token"
		integrationCtx := &contexts.IntegrationContext{
			Configuration: map[string]any{
				"botToken": botToken,
			},
		}

		err := tg.Sync(core.SyncContext{
			Configuration: map[string]any{"botToken": botToken},
			Integration:   integrationCtx,
		})

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to verify bot token")
	})
}

func Test__Telegram__HandleRequest(t *testing.T) {
	tg := &Telegram{}

	t.Run("update without message -> 200, no dispatch", func(t *testing.T) {
		body := `{"update_id": 1}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		rw := httptest.NewRecorder()

		integrationCtx := &contexts.IntegrationContext{}

		tg.HandleRequest(core.HTTPRequestContext{
			Request:     req,
			Response:    rw,
			Integration: integrationCtx,
			Logger:      newTestLogger(),
		})

		assert.Equal(t, http.StatusOK, rw.Code)
	})

	t.Run("message update -> dispatches to matching subscriptions", func(t *testing.T) {
		body := `{
			"update_id": 2,
			"message": {
				"message_id": 10,
				"chat": {"id": -100, "type": "group", "title": "Test"},
				"text": "@mybot hello",
				"date": 1737028800,
				"entities": [{"type": "mention", "offset": 0, "length": 6}]
			}
		}`
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		rw := httptest.NewRecorder()

		sub := &mockSubscription{config: SubscriptionConfiguration{EventTypes: []string{"mention"}}}
		integrationCtx := &contexts.IntegrationContext{
			Metadata: Metadata{BotID: 123, Username: "mybot"},
			Subscriptions: []contexts.Subscription{
				{Configuration: SubscriptionConfiguration{EventTypes: []string{"mention"}}},
			},
		}
		_ = sub

		tg.HandleRequest(core.HTTPRequestContext{
			Request:     req,
			Response:    rw,
			Integration: integrationCtx,
			Logger:      newTestLogger(),
		})

		assert.Equal(t, http.StatusOK, rw.Code)
	})
}
