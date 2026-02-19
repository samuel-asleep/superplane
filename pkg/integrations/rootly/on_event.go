package rootly

import (
	"encoding/json"
	"fmt"
	"net/http"
	"slices"

	"github.com/mitchellh/mapstructure"
	"github.com/superplanehq/superplane/pkg/configuration"
	"github.com/superplanehq/superplane/pkg/core"
)

type OnEvent struct{}

type OnEventConfiguration struct {
	IncidentStatus []string `json:"incidentStatus,omitempty"`
	Severity       []string `json:"severity,omitempty"`
	Service        []string `json:"service,omitempty"`
	Team           []string `json:"team,omitempty"`
	EventSource    *string  `json:"eventSource,omitempty"`
	Visibility     []string `json:"visibility,omitempty"`
	EventKind      []string `json:"eventKind,omitempty"`
}

func (t *OnEvent) Name() string {
	return "rootly.onEvent"
}

func (t *OnEvent) Label() string {
	return "On Event"
}

func (t *OnEvent) Description() string {
	return "Listen to incident timeline events"
}

func (t *OnEvent) Documentation() string {
	return `The On Event trigger starts a workflow execution when Rootly incident timeline events occur.

## Use Cases

- **Timeline automation**: Automate responses when incident notes or events are added
- **Event monitoring**: Monitor specific types of events or notes in incident timelines
- **Notification workflows**: Send notifications when important timeline events happen
- **Integration workflows**: Sync incident timeline events with external systems

## Configuration

- **Incident Status**: Filter by incident status (e.g., started, mitigated, resolved)
- **Severity**: Filter by incident severity (e.g., sev1, sev2)
- **Service**: Filter by service name
- **Team**: Filter by team name
- **Event Source**: Filter by event source
- **Visibility**: Filter by event visibility (internal/external)
- **Event Kind**: Filter by event kind (e.g., note)

## Event Data

Each incident event includes:
- **id**: Event ID
- **event**: Event text content
- **kind**: Event type (note, status change, etc.)
- **visibility**: Event visibility (internal/external)
- **occurred_at**: When the event occurred
- **created_at**: When the event was created
- **user_display_name**: User who created the event
- **event_source**: Source of the event
- **incident**: Complete incident information including title, status, severity, services, teams

## Webhook Setup

This trigger automatically sets up a Rootly webhook endpoint when configured. The endpoint is managed by SuperPlane and will be cleaned up when the trigger is removed.`
}

func (t *OnEvent) Icon() string {
	return "file-text"
}

func (t *OnEvent) Color() string {
	return "gray"
}

func (t *OnEvent) Configuration() []configuration.Field {
	return []configuration.Field{
		{
			Name:     "incidentStatus",
			Label:    "Incident Status",
			Type:     configuration.FieldTypeMultiSelect,
			Required: false,
			TypeOptions: &configuration.TypeOptions{
				MultiSelect: &configuration.MultiSelectTypeOptions{
					Options: []configuration.FieldOption{
						{Label: "In Triage", Value: "in_triage"},
						{Label: "Started", Value: "started"},
						{Label: "Detected", Value: "detected"},
						{Label: "Acknowledged", Value: "acknowledged"},
						{Label: "Mitigated", Value: "mitigated"},
						{Label: "Resolved", Value: "resolved"},
						{Label: "Closed", Value: "closed"},
						{Label: "Cancelled", Value: "cancelled"},
					},
				},
			},
		},
		{
			Name:     "severity",
			Label:    "Severity",
			Type:     configuration.FieldTypeIntegrationResource,
			Required: false,
			TypeOptions: &configuration.TypeOptions{
				Resource: &configuration.ResourceTypeOptions{
					Type:  "severity",
					Multi: true,
				},
			},
		},
		{
			Name:     "service",
			Label:    "Service",
			Type:     configuration.FieldTypeIntegrationResource,
			Required: false,
			TypeOptions: &configuration.TypeOptions{
				Resource: &configuration.ResourceTypeOptions{
					Type:  "service",
					Multi: true,
				},
			},
		},
		{
			Name:     "team",
			Label:    "Team",
			Type:     configuration.FieldTypeIntegrationResource,
			Required: false,
			TypeOptions: &configuration.TypeOptions{
				Resource: &configuration.ResourceTypeOptions{
					Type:  "team",
					Multi: true,
				},
			},
		},
		{
			Name:     "eventSource",
			Label:    "Event Source",
			Type:     configuration.FieldTypeString,
			Required: false,
		},
		{
			Name:     "visibility",
			Label:    "Visibility",
			Type:     configuration.FieldTypeMultiSelect,
			Required: false,
			TypeOptions: &configuration.TypeOptions{
				MultiSelect: &configuration.MultiSelectTypeOptions{
					Options: []configuration.FieldOption{
						{Label: "Internal", Value: "internal"},
						{Label: "External", Value: "external"},
					},
				},
			},
		},
		{
			Name:     "eventKind",
			Label:    "Event Kind",
			Type:     configuration.FieldTypeMultiSelect,
			Required: false,
			TypeOptions: &configuration.TypeOptions{
				MultiSelect: &configuration.MultiSelectTypeOptions{
					Options: []configuration.FieldOption{
						{Label: "Note", Value: "note"},
						{Label: "Status Change", Value: "status_change"},
						{Label: "Detection", Value: "detection"},
						{Label: "Mitigation", Value: "mitigation"},
						{Label: "Resolution", Value: "resolution"},
						{Label: "Alert", Value: "alert"},
						{Label: "Escalation", Value: "escalation"},
					},
				},
			},
		},
	}
}

func (t *OnEvent) Setup(ctx core.TriggerContext) error {
	config := OnEventConfiguration{}
	err := mapstructure.Decode(ctx.Configuration, &config)
	if err != nil {
		return fmt.Errorf("failed to decode configuration: %w", err)
	}

	return ctx.Integration.RequestWebhook(WebhookConfiguration{
		Events: []string{"incident_event.created", "incident_event.updated"},
	})
}

func (t *OnEvent) Actions() []core.Action {
	return []core.Action{}
}

func (t *OnEvent) HandleAction(ctx core.TriggerActionContext) (map[string]any, error) {
	return nil, nil
}

func (t *OnEvent) HandleWebhook(ctx core.WebhookRequestContext) (int, error) {
	config := OnEventConfiguration{}
	err := mapstructure.Decode(ctx.Configuration, &config)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("failed to decode configuration: %w", err)
	}

	// Verify signature
	signature := ctx.Headers.Get("X-Rootly-Signature")
	secret, err := ctx.Webhook.GetSecret()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error getting secret: %v", err)
	}

	if err := verifyWebhookSignature(signature, ctx.Body, secret); err != nil {
		return http.StatusForbidden, fmt.Errorf("invalid signature: %v", err)
	}

	// Parse webhook payload
	var webhook WebhookPayload
	err = json.Unmarshal(ctx.Body, &webhook)
	if err != nil {
		return http.StatusBadRequest, fmt.Errorf("error parsing request body: %v", err)
	}

	eventType := webhook.Event.Type

	// Only handle incident event types
	if eventType != "incident_event.created" && eventType != "incident_event.updated" {
		return http.StatusOK, nil
	}

	// Decode integration metadata for resource ID resolution (e.g. severity UUID → slug)
	var metadata Metadata
	if ctx.Integration != nil {
		mapstructure.Decode(ctx.Integration.GetMetadata(), &metadata) //nolint:errcheck
	}

	// Apply filters
	if !matchesFilters(webhook.Data, config, metadata) {
		return http.StatusOK, nil
	}

	err = ctx.Events.Emit(
		fmt.Sprintf("rootly.%s", eventType),
		buildEventPayload(webhook),
	)

	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error emitting event: %v", err)
	}

	return http.StatusOK, nil
}

func matchesFilters(data map[string]any, config OnEventConfiguration, metadata Metadata) bool {
	// Filter by visibility
	if len(config.Visibility) > 0 {
		visibility, _ := data["visibility"].(string)
		if !slices.Contains(config.Visibility, visibility) {
			return false
		}
	}

	// Filter by event kind
	if len(config.EventKind) > 0 {
		kind, _ := data["kind"].(string)
		if !slices.Contains(config.EventKind, kind) {
			return false
		}
	}

	// Filter by event source
	if config.EventSource != nil {
		eventSource, _ := data["event_source"].(string)
		if eventSource != *config.EventSource {
			return false
		}
	}

	// Check incident filters
	incident, ok := data["incident"].(map[string]any)
	if !ok {
		return true // No incident data to filter on
	}

	// Filter by incident status
	if len(config.IncidentStatus) > 0 {
		status, _ := incident["status"].(string)
		if !slices.Contains(config.IncidentStatus, status) {
			return false
		}
	}

	// Filter by severity — config values may be severity slugs (direct) or resource IDs
	// (when selected via the integration resource picker). In the latter case we resolve
	// the IDs to slugs using the cached metadata.
	if len(config.Severity) > 0 {
		webhookSeverity := severityString(incident["severity"])
		matched := false
		for _, configSeverity := range config.Severity {
			if webhookSeverity == configSeverity {
				matched = true
				break
			}
			for _, sev := range metadata.Severities {
				if sev.ID == configSeverity && (sev.Slug == webhookSeverity || sev.Name == webhookSeverity) {
					matched = true
					break
				}
			}
			if matched {
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Filter by service — config values may be service names, slugs, or resource IDs
	if len(config.Service) > 0 {
		services, ok := incident["services"].([]any)
		if !ok {
			return false
		}
		found := false
		for _, svc := range services {
			svcMap, ok := svc.(map[string]any)
			if !ok {
				continue
			}
			name, _ := svcMap["name"].(string)
			slug, _ := svcMap["slug"].(string)
			id, _ := svcMap["id"].(string)
			if slices.Contains(config.Service, name) || slices.Contains(config.Service, slug) || slices.Contains(config.Service, id) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Filter by team (groups) — config values may be team names, slugs, or resource IDs
	if len(config.Team) > 0 {
		groups, ok := incident["groups"].([]any)
		if !ok {
			return false
		}
		found := false
		for _, grp := range groups {
			grpMap, ok := grp.(map[string]any)
			if !ok {
				continue
			}
			name, _ := grpMap["name"].(string)
			slug, _ := grpMap["slug"].(string)
			id, _ := grpMap["id"].(string)
			if slices.Contains(config.Team, name) || slices.Contains(config.Team, slug) || slices.Contains(config.Team, id) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

func buildEventPayload(webhook WebhookPayload) map[string]any {
	payload := map[string]any{}

	// Copy relevant fields from webhook.Data
	if webhook.Data != nil {
		if id, ok := webhook.Data["id"]; ok {
			payload["id"] = id
		}
		if event, ok := webhook.Data["event"]; ok {
			payload["event"] = event
		}
		if kind, ok := webhook.Data["kind"]; ok {
			payload["kind"] = kind
		}
		if visibility, ok := webhook.Data["visibility"]; ok {
			payload["visibility"] = visibility
		}
		if occurredAt, ok := webhook.Data["occurred_at"]; ok {
			payload["occurred_at"] = occurredAt
		}
		if createdAt, ok := webhook.Data["created_at"]; ok {
			payload["created_at"] = createdAt
		}
		if userDisplayName, ok := webhook.Data["user_display_name"]; ok {
			payload["user_display_name"] = userDisplayName
		}
		if eventSource, ok := webhook.Data["event_source"]; ok {
			payload["event_source"] = eventSource
		}
		if incident, ok := webhook.Data["incident"]; ok {
			payload["incident"] = incident
		}
	}

	return payload
}

func (t *OnEvent) Cleanup(ctx core.TriggerContext) error {
	return nil
}
