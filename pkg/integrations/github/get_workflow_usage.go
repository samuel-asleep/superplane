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
	Repositories []string `mapstructure:"repositories"`
}

type GetWorkflowUsageMetadata struct {
	Repositories []RepositoryMetadata `json:"repositories" mapstructure:"repositories"`
}

type RepositoryMetadata struct {
	ID   int64  `json:"id" mapstructure:"id"`
	Name string `json:"name" mapstructure:"name"`
	URL  string `json:"url" mapstructure:"url"`
}

type WorkflowUsageResult struct {
	MinutesUsed          float64                 `json:"minutes_used" mapstructure:"minutes_used"`
	MinutesUsedBreakdown gh.MinutesUsedBreakdown `json:"minutes_used_breakdown" mapstructure:"minutes_used_breakdown"`
	IncludedMinutes      float64                 `json:"included_minutes" mapstructure:"included_minutes"`
	TotalPaidMinutesUsed float64                 `json:"total_paid_minutes_used" mapstructure:"total_paid_minutes_used"`
	Repositories         []string                `json:"repositories,omitempty" mapstructure:"repositories,omitempty"`
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

This action calls GitHub's **billing usage** API, which requires:

1. The GitHub App to have **Organization permission: Organization administration (read)**
2. **GitHub Enhanced Billing Platform**: As of early 2025, GitHub deprecated the old billing APIs. Your organization must be migrated to GitHub's enhanced billing platform for this component to work.

**Important**: 
- Existing installations will need to approve the new permission when prompted by GitHub
- If you receive a 410 error, your organization needs to migrate to the enhanced billing platform
- Until the permission is granted, this action will return a 403 error

## Behavior

- Returns billing data for the **current billing cycle** 
- Only private repositories on GitHub-hosted runners accrue billable minutes
- Public repositories and self-hosted runners show zero billable usage
- Returns organization-wide usage data
- Note: Repository selection is for reference/tracking only; the API returns org-wide totals

## Configuration

- **Repositories** (optional, multiselect): Select one or more specific repositories to track. These will be included in the output for reference (max 5) and stored in node metadata with full repository details (ID, name, URL). Note: The usage data returned is organization-wide and not filtered by repository.

## Output

Returns usage data with:
- ` + "`minutes_used`" + `: Total billable minutes used in the current billing cycle
- ` + "`minutes_used_breakdown`" + `: Map of minutes by runner OS (e.g., "UBUNTU": 120, "WINDOWS": 60, "MACOS": 30)
- ` + "`included_minutes`" + `: Number of free minutes included in the plan
- ` + "`total_paid_minutes_used`" + `: Total paid minutes (beyond included minutes)
- ` + "`repositories`" + `: List of selected repositories for tracking (max 5)

**Note**: Breakdown is by runner OS type, not by individual workflow or repository.

## Node Metadata

The component stores repository information in node metadata:
- Repository ID, name, and URL for each selected repository (max 5)
- This metadata is displayed in the workflow canvas for easy reference

## Use Cases

- **Billing Monitoring**: Track GitHub Actions usage for billing purposes
- **Quota Management**: Monitor usage to avoid exceeding billing quotas
- **Cost Control**: Alert when usage approaches limits or budget thresholds
- **Usage Reporting**: Generate monthly or periodic usage reports for compliance
- **Resource Planning**: Analyze runner usage patterns by OS type

## References

- [GitHub Billing Usage API](https://docs.github.com/rest/billing/usage)
- [GitHub Enhanced Billing Platform](https://docs.github.com/billing/using-the-new-billing-platform)
- [Permissions required for GitHub Apps - Organization Administration](https://docs.github.com/en/rest/overview/permissions-required-for-github-apps#organization-permissions-for-administration)
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
	return []configuration.Field{
		{
			Name:        "repositories",
			Label:       "Repositories",
			Type:        configuration.FieldTypeIntegrationResource,
			Required:    false,
			Description: "Select specific repositories to check usage for. Leave empty for organization-wide usage.",
			TypeOptions: &configuration.TypeOptions{
				Resource: &configuration.ResourceTypeOptions{
					Type:           "repository",
					UseNameAsValue: true,
					Multi:          true,
				},
			},
		},
	}
}

func (g *GetWorkflowUsage) Setup(ctx core.SetupContext) error {
	var config GetWorkflowUsageConfiguration
	if err := mapstructure.Decode(ctx.Configuration, &config); err != nil {
		return fmt.Errorf("failed to decode configuration: %w", err)
	}

	// If repositories are specified, validate they exist in metadata
	if len(config.Repositories) > 0 {
		// Validate each repository
		var appMetadata Metadata
		if err := mapstructure.Decode(ctx.Integration.GetMetadata(), &appMetadata); err != nil {
			return fmt.Errorf("failed to decode application metadata: %w", err)
		}

		// Check each repository exists and collect full repository objects
		var selectedRepos []RepositoryMetadata
		for _, repoName := range config.Repositories {
			found := false
			for _, availableRepo := range appMetadata.Repositories {
				if availableRepo.Name == repoName {
					selectedRepos = append(selectedRepos, RepositoryMetadata{
						ID:   availableRepo.ID,
						Name: availableRepo.Name,
						URL:  availableRepo.URL,
					})
					found = true
					break
				}
			}
			if !found {
				return fmt.Errorf("repository %s is not accessible to app installation", repoName)
			}
		}

		// Save selected repositories to node metadata (max 5)
		reposToStore := selectedRepos
		if len(reposToStore) > 5 {
			reposToStore = reposToStore[:5]
		}

		metadata := GetWorkflowUsageMetadata{
			Repositories: reposToStore,
		}

		return ctx.Metadata.Set(metadata)
	}

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

	// Use the standard billing API which works for all organizations
	// NOTE: As of early 2025, GitHub deprecated the old billing endpoints in favor
	// of the enhanced billing platform. Organizations need to be on the enhanced
	// billing platform for this API to work. If you get a 410 error, your org needs
	// to migrate to enhanced billing.
	billing, _, err := client.Billing.GetActionsBillingOrg(
		context.Background(),
		appMetadata.Owner,
	)
	if err != nil {
		return fmt.Errorf("failed to get billing usage (organization may need to migrate to GitHub's enhanced billing platform): %w", err)
	}

	result := WorkflowUsageResult{
		MinutesUsed:          billing.TotalMinutesUsed,
		MinutesUsedBreakdown: billing.MinutesUsedBreakdown,
		IncludedMinutes:      billing.IncludedMinutes,
		TotalPaidMinutesUsed: billing.TotalPaidMinutesUsed,
	}

	// Add selected repositories to output (max 5)
	if len(config.Repositories) > 0 {
		reposToInclude := config.Repositories
		if len(reposToInclude) > 5 {
			reposToInclude = reposToInclude[:5]
		}
		result.Repositories = reposToInclude
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
