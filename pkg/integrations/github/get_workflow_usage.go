package github

import (
	"context"
	_ "embed"
	"fmt"
	"sync"

	gh "github.com/google/go-github/v74/github"
	"github.com/google/uuid"
	"github.com/mitchellh/mapstructure"
	"github.com/superplanehq/superplane/pkg/configuration"
	"github.com/superplanehq/superplane/pkg/core"
	"github.com/superplanehq/superplane/pkg/utils"
)

type GetWorkflowUsage struct{}

//go:embed get_workflow_usage_example_output.json
var getWorkflowUsageExampleOutputBytes []byte

var getWorkflowUsageExampleOutputOnce sync.Once
var getWorkflowUsageExampleOutput map[string]any

type GetWorkflowUsageConfiguration struct {
}

type WorkflowUsageResult struct {
	MinutesUsed          float64                 `json:"minutes_used" mapstructure:"minutes_used"`
	MinutesUsedBreakdown gh.MinutesUsedBreakdown `json:"minutes_used_breakdown" mapstructure:"minutes_used_breakdown"`
	IncludedMinutes      float64                 `json:"included_minutes" mapstructure:"included_minutes"`
	TotalPaidMinutesUsed float64                 `json:"total_paid_minutes_used" mapstructure:"total_paid_minutes_used"`
}

func (g *GetWorkflowUsage) Name() string {
	return "github.getWorkflowUsage"
}

func (g *GetWorkflowUsage) Label() string {
	return "Get Workflow Usage"
}

func (g *GetWorkflowUsage) Description() string {
	return "Retrieve billable GitHub Actions usage (minutes) for the organization"
}

func (g *GetWorkflowUsage) Documentation() string {
	return `The Get Workflow Usage component retrieves billable GitHub Actions usage (minutes) for the installation's organization.

## Prerequisites

This action calls GitHub's **billing usage** API, which requires the GitHub App to have **Organization permission: Administration (read)**. 

**Important**: Existing installations will need to approve the new permission when prompted by GitHub. Until the permission is granted, this action will return a 403 error.

## Behavior

- Returns billing data for the **current billing cycle** only
- Only private repositories on GitHub-hosted runners accrue billable minutes
- Public repositories and self-hosted runners show zero billable usage
- Usage is aggregated at the organization level
- Cannot filter by specific repositories, time period, or runner OS (returns all data for current cycle)

## Configuration

No configuration fields required - this component automatically retrieves organization-wide billing data.

## Output

Returns usage data with:
- ` + "`minutes_used`" + `: Total billable minutes used in the current billing cycle
- ` + "`minutes_used_breakdown`" + `: Map of minutes by runner OS (e.g., "UBUNTU": 120, "WINDOWS": 60, "MACOS": 30)
- ` + "`included_minutes`" + `: Number of free minutes included in the plan
- ` + "`total_paid_minutes_used`" + `: Total paid minutes (beyond included minutes)

**Note**: Breakdown is by OS (runner type), not by individual workflow or repository.

## Use Cases

- Check Actions usage for billing or quota from SuperPlane workflows
- Report on workflow run minutes for cost or compliance
- Alert when usage approaches limits (by comparing to a threshold in a later node)
- Monitor paid usage to control costs

## References

- [GitHub Billing Usage API](https://docs.github.com/rest/billing/usage)
- [Permissions required for GitHub Apps - Administration](https://docs.github.com/en/rest/overview/permissions-required-for-github-apps#organization-permissions-for-administration)
- [Viewing your usage of metered products](https://docs.github.com/en/billing/managing-billing-for-github-actions/viewing-your-github-actions-usage)`
}

func (g *GetWorkflowUsage) Icon() string {
	return "github"
}

func (g *GetWorkflowUsage) Color() string {
	return "gray"
}

func (g *GetWorkflowUsage) OutputChannels(configuration any) []core.OutputChannel {
	return []core.OutputChannel{core.DefaultOutputChannel}
}

func (g *GetWorkflowUsage) Configuration() []configuration.Field {
	return []configuration.Field{}
}

func (g *GetWorkflowUsage) Setup(ctx core.SetupContext) error {
	// Repositories are optional, so we don't enforce repo validation here
	return nil
}

func (g *GetWorkflowUsage) Execute(ctx core.ExecutionContext) error {
	var config GetWorkflowUsageConfiguration
	if err := mapstructure.Decode(ctx.Configuration, &config); err != nil {
		return fmt.Errorf("failed to decode configuration: %w", err)
	}

	var appMetadata Metadata
	if err := mapstructure.Decode(ctx.Integration.GetMetadata(), &appMetadata); err != nil {
		return fmt.Errorf("failed to decode application metadata: %w", err)
	}

	client, err := NewClient(ctx.Integration, appMetadata.GitHubApp.ID, appMetadata.InstallationID)
	if err != nil {
		return fmt.Errorf("failed to initialize GitHub client: %w", err)
	}

	// Get organization-wide billing information
	// Note: This API returns the current billing cycle data
	billing, _, err := client.Billing.GetActionsBillingOrg(
		context.Background(),
		appMetadata.Owner,
	)
	if err != nil {
		return fmt.Errorf("failed to get billing usage: %w", err)
	}

	result := WorkflowUsageResult{
		MinutesUsed:          billing.TotalMinutesUsed,
		MinutesUsedBreakdown: billing.MinutesUsedBreakdown,
		IncludedMinutes:      billing.IncludedMinutes,
		TotalPaidMinutesUsed: billing.TotalPaidMinutesUsed,
	}

	return ctx.ExecutionState.Emit(
		core.DefaultOutputChannel.Name,
		"github.workflowUsage",
		[]any{result},
	)
}

func (g *GetWorkflowUsage) ProcessQueueItem(ctx core.ProcessQueueContext) (*uuid.UUID, error) {
	return ctx.DefaultProcessing()
}

func (g *GetWorkflowUsage) HandleWebhook(ctx core.WebhookRequestContext) (int, error) {
	return 200, nil
}

func (g *GetWorkflowUsage) Actions() []core.Action {
	return []core.Action{}
}

func (g *GetWorkflowUsage) HandleAction(ctx core.ActionContext) error {
	return nil
}

func (g *GetWorkflowUsage) Cancel(ctx core.ExecutionContext) error {
	return nil
}

func (g *GetWorkflowUsage) Cleanup(ctx core.SetupContext) error {
	return nil
}

func (g *GetWorkflowUsage) ExampleOutput() map[string]any {
	return utils.UnmarshalEmbeddedJSON(&getWorkflowUsageExampleOutputOnce, getWorkflowUsageExampleOutputBytes, &getWorkflowUsageExampleOutput)
}
