package rootly

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/superplanehq/superplane/pkg/core"
	"github.com/superplanehq/superplane/test/support/contexts"
)

func Test__OnEvent__Setup(t *testing.T) {
	trigger := &OnEvent{}

	t.Run("invalid configuration -> decode error", func(t *testing.T) {
		integrationCtx := &contexts.IntegrationContext{}
		err := trigger.Setup(core.TriggerContext{
			Integration:   integrationCtx,
			Configuration: "invalid-config",
		})

		require.ErrorContains(t, err, "failed to decode configuration")
	})

	t.Run("valid configuration with no filters -> webhook request", func(t *testing.T) {
		integrationCtx := &contexts.IntegrationContext{}
		err := trigger.Setup(core.TriggerContext{
			Integration:   integrationCtx,
			Configuration: OnEventConfiguration{},
		})

		require.NoError(t, err)
		require.Len(t, integrationCtx.WebhookRequests, 1)

		webhookConfig := integrationCtx.WebhookRequests[0].(WebhookConfiguration)
		assert.Equal(t, []string{"incident_event.created", "incident_event.updated"}, webhookConfig.Events)
	})

	t.Run("valid configuration with filters -> webhook request", func(t *testing.T) {
		integrationCtx := &contexts.IntegrationContext{}
		err := trigger.Setup(core.TriggerContext{
			Integration: integrationCtx,
			Configuration: OnEventConfiguration{
				Visibility: []string{"internal"},
				EventKind:  []string{"note"},
			},
		})

		require.NoError(t, err)
		require.Len(t, integrationCtx.WebhookRequests, 1)

		webhookConfig := integrationCtx.WebhookRequests[0].(WebhookConfiguration)
		assert.Equal(t, []string{"incident_event.created", "incident_event.updated"}, webhookConfig.Events)
	})
}

func Test__OnEvent__HandleWebhook(t *testing.T) {
	trigger := &OnEvent{}

	validConfig := map[string]any{}

	signatureFor := func(secret string, timestamp string, body []byte) string {
		payload := append([]byte(timestamp), body...)
		sig := computeHMACSHA256([]byte(secret), payload)
		return "t=" + timestamp + ",v1=" + sig
	}

	t.Run("missing X-Rootly-Signature -> 403", func(t *testing.T) {
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Headers:       http.Header{},
			Configuration: validConfig,
			Webhook:       &contexts.WebhookContext{Secret: "test-secret"},
			Events:        &contexts.EventContext{},
		})

		assert.Equal(t, http.StatusForbidden, code)
		assert.ErrorContains(t, err, "invalid signature")
	})

	t.Run("invalid signature -> 403", func(t *testing.T) {
		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123"}}`)
		headers := http.Header{}
		headers.Set("X-Rootly-Signature", "t=1234567890,v1=invalid")

		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: validConfig,
			Webhook:       &contexts.WebhookContext{Secret: "test-secret"},
			Events:        &contexts.EventContext{},
		})

		assert.Equal(t, http.StatusForbidden, code)
		assert.ErrorContains(t, err, "invalid signature")
	})

	t.Run("invalid JSON body -> 400", func(t *testing.T) {
		body := []byte("invalid json")
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: validConfig,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        &contexts.EventContext{},
		})

		assert.Equal(t, http.StatusBadRequest, code)
		assert.ErrorContains(t, err, "error parsing request body")
	})

	t.Run("non-incident-event event type -> no emit", func(t *testing.T) {
		body := []byte(`{"event":{"type":"incident.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"inc-123","title":"Test Incident"}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: validConfig,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		assert.Equal(t, http.StatusOK, code)
		assert.NoError(t, err)
		assert.Equal(t, 0, eventContext.Count())
	})

	t.Run("incident_event.created with no filters -> event emitted", func(t *testing.T) {
		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test note","kind":"note","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1","services":[{"id":"svc-1","name":"prod-api","slug":"prod-api"}],"groups":[{"id":"grp-1","name":"platform","slug":"platform"}]}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: validConfig,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 1, eventContext.Count())

		payload := eventContext.Payloads[0]
		assert.Equal(t, "rootly.incident_event.created", payload.Type)
		assert.Equal(t, "ie-123", payload.Data.(map[string]any)["id"])
		assert.Equal(t, "Test note", payload.Data.(map[string]any)["event"])
		assert.NotNil(t, payload.Data.(map[string]any)["incident"])
	})

	t.Run("incident_event.created filtered by visibility -> emitted when matches", func(t *testing.T) {
		config := map[string]any{
			"visibility": []string{"internal"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test note","kind":"note","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1","services":[{"id":"svc-1","name":"prod-api","slug":"prod-api"}],"groups":[{"id":"grp-1","name":"platform","slug":"platform"}]}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 1, eventContext.Count())
	})

	t.Run("incident_event.created filtered by visibility -> not emitted when not matching", func(t *testing.T) {
		config := map[string]any{
			"visibility": []string{"internal"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test note","kind":"note","visibility":"external","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1"}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 0, eventContext.Count())
	})

	t.Run("incident_event.created filtered by eventKind -> emitted when matches", func(t *testing.T) {
		config := map[string]any{
			"eventKind": []string{"note"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test note","kind":"note","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1"}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 1, eventContext.Count())
	})

	t.Run("incident_event.created filtered by eventKind -> not emitted when not matching", func(t *testing.T) {
		config := map[string]any{
			"eventKind": []string{"note"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test status change","kind":"status_change","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1"}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 0, eventContext.Count())
	})

	t.Run("incident_event.created filtered by incidentStatus -> emitted when matches", func(t *testing.T) {
		config := map[string]any{
			"incidentStatus": []string{"started"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test note","kind":"note","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1"}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 1, eventContext.Count())
	})

	t.Run("incident_event.created filtered by severity -> emitted when matches", func(t *testing.T) {
		config := map[string]any{
			"severity": []string{"sev1"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test note","kind":"note","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1"}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 1, eventContext.Count())
	})

	t.Run("incident_event.created filtered by service -> emitted when service name matches", func(t *testing.T) {
		config := map[string]any{
			"service": []string{"prod-api"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test note","kind":"note","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1","services":[{"id":"svc-1","name":"prod-api","slug":"prod-api"}]}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 1, eventContext.Count())
	})

	t.Run("incident_event.created filtered by team -> emitted when team name matches", func(t *testing.T) {
		config := map[string]any{
			"team": []string{"platform"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test note","kind":"note","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1","groups":[{"id":"grp-1","name":"platform","slug":"platform"}]}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 1, eventContext.Count())
	})

	t.Run("filtered by service -> emitted when resource ID matches", func(t *testing.T) {
		config := map[string]any{
			"service": []string{"svc-1"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test note","kind":"note","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1","services":[{"id":"svc-1","name":"prod-api","slug":"prod-api"}]}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 1, eventContext.Count())
	})

	t.Run("filtered by team -> emitted when resource ID matches", func(t *testing.T) {
		config := map[string]any{
			"team": []string{"grp-1"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test note","kind":"note","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1","groups":[{"id":"grp-1","name":"platform","slug":"platform"}]}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 1, eventContext.Count())
	})

	t.Run("filtered by severity -> emitted when resource ID matches via metadata", func(t *testing.T) {
		config := map[string]any{
			"severity": []string{"sev-uuid-123"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test note","kind":"note","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1"}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		integrationCtx := &contexts.IntegrationContext{
			Metadata: Metadata{
				Severities: []Severity{
					{ID: "sev-uuid-123", Name: "SEV1", Slug: "sev1"},
				},
			},
		}

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
			Integration:   integrationCtx,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 1, eventContext.Count())
	})

	t.Run("filtered by severity -> not emitted when resource ID does not match", func(t *testing.T) {
		config := map[string]any{
			"severity": []string{"sev-uuid-456"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test note","kind":"note","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1"}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		integrationCtx := &contexts.IntegrationContext{
			Metadata: Metadata{
				Severities: []Severity{
					{ID: "sev-uuid-123", Name: "SEV1", Slug: "sev1"},
				},
			},
		}

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
			Integration:   integrationCtx,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 0, eventContext.Count())
	})

	t.Run("multi-select eventKind -> emitted when event matches any selected kind", func(t *testing.T) {
		config := map[string]any{
			"eventKind": []string{"note", "status_change"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Status changed","kind":"status_change","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1"}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 1, eventContext.Count())
	})

	t.Run("multi-select service -> emitted when incident service matches any selected service", func(t *testing.T) {
		config := map[string]any{
			"service": []string{"other-service", "prod-api"},
		}

		body := []byte(`{"event":{"type":"incident_event.created","id":"evt-123","issued_at":"2025-01-01T00:00:00Z"},"data":{"id":"ie-123","event":"Test note","kind":"note","visibility":"internal","occurred_at":"2025-01-01T00:00:00Z","created_at":"2025-01-01T00:00:00Z","user_display_name":"Jane Doe","incident":{"id":"inc-123","title":"Test Incident","status":"started","severity":"sev1","services":[{"id":"svc-1","name":"prod-api","slug":"prod-api"}]}}}`)
		secret := "test-secret"
		timestamp := "1234567890"

		headers := http.Header{}
		headers.Set("X-Rootly-Signature", signatureFor(secret, timestamp, body))

		eventContext := &contexts.EventContext{}
		code, err := trigger.HandleWebhook(core.WebhookRequestContext{
			Body:          body,
			Headers:       headers,
			Configuration: config,
			Webhook:       &contexts.WebhookContext{Secret: secret},
			Events:        eventContext,
		})

		require.Equal(t, http.StatusOK, code)
		require.NoError(t, err)
		require.Equal(t, 1, eventContext.Count())
	})
}
