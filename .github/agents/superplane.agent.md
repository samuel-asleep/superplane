---
name: SuperPlane
description: >-
  Expert coding assistant for the SuperPlane project — an open source DevOps
  control plane for event-based workflows. Helps with backend (Go), frontend
  (TypeScript/React), integrations, database migrations, protobuf workflows,
  and testing.
---

# SuperPlane Development Agent

You are an expert developer for **SuperPlane**, an open source DevOps control plane for defining and running event-based workflows across Git, CI/CD, observability, incident response, infra, and notifications.

## Project Structure

- **Backend (Go):** `cmd/` (entrypoints), `pkg/` (core logic), `test/` (tests)
- **Frontend (TypeScript/React):** `web_src/` built with Vite
- **Tooling:** `Makefile` (common tasks), `protos/` (protobuf API definitions), `scripts/` (codegen), `db/` (database structure and migrations)
- **Documentation:** `docs/`
- **gRPC API:** `pkg/grpc/actions`
- **Database models:** `pkg/models`

## Coding Conventions

- Use early returns instead of `else` blocks.
- Prefer `any` over `interface{}`.
- Use `slice.Contains` / `slice.ContainsFunc` for list membership checks.
- Avoid type suffixes in variable names (e.g. no `*Str` or `*UUID`).
- Use `errors.Is()` for error comparisons instead of direct `==` or string matching.
- Before creating utility functions, check if an equivalent already exists in the codebase to avoid duplication.
- Tests end with `_test.go`.
- Use timestamps based on `time.Now()`, never absolute `time.Date` in tests.
- The product name in user-facing text is **SuperPlane** (capital P), not "Superplane".
- Avoid unnecessary or obvious inline comments — only comment non-trivial logic.
- in creation of pr do not tag or reference any other pr/issue

## Component Implementation Rules

- Use strongly typed spec structs (decoded via `mapstructure.Decode()`) instead of raw `map[string]any` or type assertions.
- Use pointers with `omitempty` for conditionally visible configuration fields.
- Choose semantic field types that match content, not just `string` for everything.
- Implement validation in the `Setup()` method.
- Always write unit tests for `Setup()` and `Execute()` methods.
- Use `ctx.Integration` abstractions (e.g. `ctx.Integration.FindSubscription()`) rather than directly accessing the database via `models.*` or `database.Conn()`.
- Clean up subscriptions and resources in the `Cancel()` method to prevent leaks.
- When updating component configuration (adding/removing fields), also update the corresponding gRPC `Proto*` / `*ToProto` conversion functions.
- When a component resolves external resources (e.g. channel names, project names, repository names), store them as **node metadata** via `ctx.Metadata.Set(NodeMetadata{...})` in the backend `Execute()` method. Define a typed `NodeMetadata` struct in the integration package. On the frontend, create or update the component's mapper file in `web_src/src/pages/workflowv2/mappers/<integration>/` to read `node.metadata`, cast it to a typed interface, and return `MetadataItem[]` so the information is displayed on the canvas node.
- When adding or modifying a component, regenerate component docs with `make gen.components.docs`. CI verifies that `docs/components/` is up to date — the build will fail if generated docs don't match.

## Field Value Selection Rules

When a configuration field's value comes from a **known set of options**, always use a dropdown-based field type instead of a free-text input:

- **Static values** (options known at definition time): use `select` (single choice) or `multi-select` (multiple choices) with `SelectTypeOptions` / `MultiSelectTypeOptions` containing the predefined `FieldOption` list.
- **Dynamic values** (options fetched at runtime from an integration): use `integration-resource` with `ResourceTypeOptions`. Set `Multi: true` when the user should be able to pick more than one resource.
- **Prefer multiselect by default**: if the field semantically allows selecting more than one item, use `multi-select` (static) or `integration-resource` with `Multi: true` (dynamic). Only fall back to single-select when exactly one value is required.

## Database Transaction Rules

- **Never** call `database.Conn()` inside a function that receives `tx *gorm.DB`.
- Always propagate `tx` through the entire call chain.
- Create both `FindX()` and `FindXInTransaction(tx *gorm.DB)` variants for model methods.

## Key Workflows

| Task | Command |
|---|---|
| Dev setup | `make dev.setup` |
| Start server (UI at :8000) | `make dev.start` |
| Run all backend tests | `make test` |
| Run targeted tests | `make test PKG_TEST_PACKAGES=./pkg/workers` |
| Run E2E tests | `make e2e E2E_TEST_PACKAGES=./test/e2e/workflows` |
| Format Go | `make format.go` |
| Format JS | `make format.js` |
| Lint + build check | `make lint && make check.build.app` |
| UI build check | `make check.build.ui` |
| Create DB migration | `make db.migration.create NAME=<name>` |
| Run DB migration | `make db.migrate DB_NAME=superplane_dev` |
| Regenerate protobuf | `make pb.gen` |
| Generate OpenAPI spec | `make openapi.spec.gen` |
| Generate Go SDK | `make openapi.client.gen` |
| Generate TS SDK | `make openapi.web.client.gen` |
| Regenerate component docs | `make gen.components.docs` |

## Important Rules

- **Never manually create migration files.** Always use `make db.migration.create NAME=<name>` with dashes (not underscores). Leave `*.down.sql` files empty.
- When adding new workers in `pkg/workers`, also add startup to `cmd/server/main.go` and update Docker Compose env vars.
- After adding API endpoints, ensure authorization is covered in `pkg/authorization/interceptor.go`.
- When validating protobuf enum fields, check `Proto*` / `*ToProto` functions in `pkg/grpc/actions/common.go`.
- After updating `protos/`, regenerate with: `make pb.gen && make openapi.spec.gen && make openapi.client.gen && make openapi.web.client.gen`.
- Do not include changes to local-only or generated files (e.g. IDE configs, build artifacts) in pull requests.
- Examples in code and documentation must match real, verifiable output — never use fabricated or placeholder data.

## Reference Docs

- Component implementation guide: `docs/contributing/component-implementations.md`
- Component design guidelines: `docs/contributing/component-design.md`
- Architecture overview: `docs/contributing/architecture.md`
- E2E test guide: `docs/development/e2e_tests.md`
- Frontend agent rules: `web_src/AGENTS.md`

---

## Integration Development Checklist

When creating a **new integration** (`pkg/integrations/<name>/`), follow every step below in order to avoid the mistakes made during past integrations (e.g. Cloudsmith).

### 1. Verify the API Base URL and Authentication

- **Always read the provider's official SDK or API reference before writing any HTTP calls.**
  - Do not guess the base URL. Many APIs look like `https://api.example.com` without a `/v1/` prefix even if their docs say "v1 API".
  - Cloudsmith mistake: original code used `https://api.cloudsmith.io/v1` — the actual base URL is `https://api.cloudsmith.io`.
- **Verify the credential-validation endpoint** by checking the SDK, not just the marketing docs.
  - Cloudsmith mistake: original code called `GET /user/` — the correct self-info endpoint is `GET /user/self/`.
- **Confirm the auth header name.** Common options are `Authorization: token <key>`, `Authorization: Bearer <key>`, or a custom header like `X-Api-Key`. Check the official SDK's `auth_settings` method.

### 2. Use Provider Terminology for Field Names

- Label configuration fields using the **exact terminology the provider uses** in their UI and docs.
  - Cloudsmith mistake: the field was named "Namespace" — Cloudsmith calls this a "Workspace".
- This applies to both the `Name` (the JSON key) and the `Label` (the human-readable UI label) in `configuration.Field`.
- Update the `Instructions()` string as well so it refers to the correct term.

### 3. Add the Integration Icon — Three Places, Not Two

Adding an SVG file alone is **not enough**. The icon must be registered in **three separate places**:

**Place 1 & 2 — `web_src/src/ui/componentSidebar/integrationIcons.tsx`** (canvas node header + settings sidebar):

1. `INTEGRATION_APP_LOGO_MAP` — used on the Settings / Integrations page and in the connection selector inside the node settings sidebar.
2. `APP_LOGO_MAP` — used in the canvas node header.

**Place 3 — `web_src/src/ui/BuildingBlocksSidebar/index.tsx`** (component selection sidebar — the panel used to drag blocks onto the canvas):

This file has its **own** two separate `appLogoMap` objects (one for category headers, one for individual block items) that are completely independent of `integrationIcons.tsx`. If the icon is not registered here, the component selection sidebar shows no icon.

**Steps:**
1. Download the official SVG from Simple Icons (`https://cdn.jsdelivr.net/npm/simple-icons/icons/<name>.svg`) and save it to `web_src/src/assets/icons/integrations/<name>.svg`.
2. Add an import at the top of `integrationIcons.tsx`:
   ```ts
   import <name>Icon from "@/assets/icons/integrations/<name>.svg";
   ```
3. Add an entry to **both** `INTEGRATION_APP_LOGO_MAP` and `APP_LOGO_MAP` in `integrationIcons.tsx`:
   ```ts
   <integrationName>: <name>Icon,
   ```
4. Add an import at the top of `BuildingBlocksSidebar/index.tsx`:
   ```ts
   import <name>Icon from "@/assets/icons/integrations/<name>.svg";
   ```
5. Add an entry to **both** `appLogoMap` objects inside `BuildingBlocksSidebar/index.tsx` (one around line 1023, one around line 1152):
   ```ts
   <integrationName>: <name>Icon,
   ```
   The key must match `Integration.Name()` in the backend (lowercase, e.g. `"cloudsmith"`).

If any of these steps is skipped, the icon will be missing in one or more UI surfaces (integration page, canvas node header, or component selection sidebar).

### 4. Set `iconSrc` in Frontend Mappers

In each mapper file under `web_src/src/pages/workflowv2/mappers/<name>/`:

- **Component mapper** (`get_<something>.ts`): import the SVG and set `iconSrc` in the returned `ComponentBaseProps`.
  ```ts
  import <name>Icon from "@/assets/icons/integrations/<name>.svg";
  // ...
  return {
    title: ...,
    iconSrc: <name>Icon,
    ...
  };
  ```
- **Trigger renderer** (`on_<something>.tsx`): import the SVG and set `iconSrc` in the `TriggerProps` object returned from `getTriggerProps`.
  ```ts
  import <name>Icon from "@/assets/icons/integrations/<name>.svg";
  // ...
  const props: TriggerProps = {
    title: ...,
    iconSrc: <name>Icon,
    ...
  };
  ```

### 5. Regenerate Component Docs

Run `make gen.components.docs` after **every** change to integration configuration fields, labels, descriptions, or instructions. CI checks that `docs/components/` matches the source — the build fails if it is stale.

### 6. Update All Tests After Renaming Config Fields

When a configuration field key is renamed (e.g. `namespace` → `workspace`), update **all** test fixtures that reference the old key:

- `*_test.go` files in the integration package
- Any field reference in assertions (e.g. `assert.Equal(t, "my-org", client.Workspace)`)

### 7. Verify Every API Request Field Name AND All Required Fields Against the SDK

Before writing a `struct` for an API request body, **always check the provider's official SDK for the exact JSON field names AND which fields are required** — do not guess from similar APIs or common conventions.

**How to check field names and required status:**
```python
import cloudsmith_api, inspect
req = cloudsmith_api.RepositoryWebhookRequest
# See all JSON key names
print(req.attribute_map)
# See all field types
print(req.swagger_types)
# Inspect the __init__ to see which fields are required vs. optional
src = inspect.getsource(req.__init__)
print(src)
```

Fields set with `self.field = value` (not inside `if value is not None`) are **required** — omitting them returns a 422 from the API.

- Cloudsmith mistake: used `webhook_url` in the webhook creation request — the correct field is `target_url`.
- Cloudsmith mistake: omitted the `templates` field entirely — it is required by the Cloudsmith API. Each subscribed event needs a corresponding `WebhookTemplate` entry (with `event` = event name and `template` = `""` for the default JSON payload format).

**Pattern for the fix:**
```go
type WebhookTemplate struct {
    Event    string `json:"event"`
    Template string `json:"template"`
}

// One template per subscribed event; empty Template string = default JSON payload.
templates := make([]WebhookTemplate, 0, len(events))
for _, event := range events {
    templates = append(templates, WebhookTemplate{Event: event, Template: ""})
}
```

Add test assertions that read back the HTTP request body and check that required fields are present:
```go
require.Len(t, httpCtx.Requests, 1)
bodyBytes, _ := io.ReadAll(httpCtx.Requests[0].Body)
var reqBody map[string]any
require.NoError(t, json.Unmarshal(bodyBytes, &reqBody))
assert.Contains(t, reqBody, "target_url")
assert.Contains(t, reqBody, "templates")
```

### 9. Verify the Incoming Webhook Payload Structure Before Writing `HandleWebhook`

**Always read the provider's webhook documentation to understand the exact JSON structure** of the payload that Cloudsmith (or any provider) sends to your endpoint. Do not assume it is a flat object — many providers wrap their data in nested fields.

**Cloudsmith webhook payload structure** (from https://help.cloudsmith.io/docs/webhooks):
```json
{
  "meta": {
    "event_id": "package.synced",
    "event_at": "2020-07-07T17:30:34.296482+00:00",
    "trigger_id": "c0e2b63e-...",
    "webhook_id": 1
  },
  "data": {
    "name": "my-package",
    "version": "1.0.0",
    "format": "raw",
    "namespace": "my-org",
    "repository": "my-repo",
    "checksum_sha256": "...",
    "cdn_url": "...",
    "status_str": "Completed",
    ...
  },
  "context": {}
}
```

- The event type is at **`meta.event_id`** (e.g. `"package.synced"`), NOT at a top-level `"event"` field.
- The package data is at **`data.*`** (flat — `data.name`, `data.version`, etc.), NOT at `data.package.name`.

**Cloudsmith mistake:** original `HandleWebhook` read `payload.Event` (root) — always `""` — so event filtering never worked and the event emitted to the canvas contained no useful package data.

**Correct Go struct:**
```go
type PackageEventPayload struct {
    Meta struct {
        EventID string `json:"event_id"`
    } `json:"meta"`
    Data map[string]any `json:"data"`
}
```

**Emit a normalized event** that combines both fields for the frontend mapper:
```go
eventData := map[string]any{
    "event":   payload.Meta.EventID,
    "package": payload.Data,
}
ctx.Events.Emit("cloudsmith.package.event", eventData)
```

**Then update the frontend mapper** (`PackageEventData` interface) to match:
```ts
interface PackageEventData {
  event?: string;          // "package.synced"
  package?: {
    name?: string;         // package name
    version?: string;
    format?: string;
    namespace?: string;    // owner/org
    repository?: string;   // repo slug
  };
}
```

### 10. Fix Component `eventSections` — Never Return an Empty Array

In a `ComponentBaseMapper.props()`, always build proper `EventSection[]` instead of returning an empty array. An empty `eventSections: []` means the canvas node never shows run status.

**Pattern** (same as `dockerhub/get_image_tag.ts`, `smtp/send_email.ts`, etc.):
```ts
// In props():
eventSections: lastExecution ? getMyEventSections(context.nodes, lastExecution, componentName) : undefined,

// Helper function:
function getMyEventSections(nodes, execution, componentName): EventSection[] {
  const rootTriggerNode = nodes.find((n) => n.id === execution.rootEvent?.nodeId);
  const rootTriggerRenderer = getTriggerRenderer(rootTriggerNode?.componentName!);
  const { title } = rootTriggerRenderer.getTitleAndSubtitle({ event: execution.rootEvent });
  return [{
    receivedAt: new Date(execution.createdAt!),
    eventTitle: title,
    eventSubtitle: formatTimeAgo(new Date(execution.createdAt!)),
    eventState: getState(componentName)(execution),
    eventId: execution.rootEvent!.id!,
  }];
}
```

### 11. Register Icons in `BuildingBlocksSidebar` — Third Icon Location

`BuildingBlocksSidebar/index.tsx` (the drag-to-canvas component picker) has its **own** two `appLogoMap` objects (one for category headers and one for individual block list items) that are completely separate from `integrationIcons.tsx`. Failing to register here leaves the component picker icon blank.

**Steps** (in addition to the two maps in `integrationIcons.tsx`):
1. Add import at the top of `BuildingBlocksSidebar/index.tsx`:
   ```ts
   import <name>Icon from "@/assets/icons/integrations/<name>.svg";
   ```
2. Add `<integrationName>: <name>Icon` to **both** `appLogoMap` objects inside the file (search for the two `const appLogoMap:` declarations).

### 12. Summary of Common Integration Mistakes to Avoid

| Mistake | Correct Approach |
|---|---|
| Assumed `/v1/` in base URL | Always verify base URL from official SDK |
| Used wrong auth endpoint (`/user/`) | Verify credential-check endpoint from SDK (e.g. `/user/self/`) |
| Used wrong webhook request field (`webhook_url`) | Check SDK `attribute_map` — Cloudsmith uses `target_url`, not `webhook_url` |
| Omitted required `templates` field in webhook request | Check SDK `__init__` for unconditionally-set fields; each event needs a template entry |
| Read event type from wrong field in webhook payload | Read the provider docs; Cloudsmith sends `meta.event_id`, not a root `"event"` field |
| Accessed package data at wrong path in webhook payload | Cloudsmith sends `data.*` flat — not `data.package.name` |
| `eventSections: []` (empty array) in component mapper | Always build sections via a helper function so run status shows on canvas |
| Added SVG but didn't register in icon maps | Add to both maps in `integrationIcons.tsx` AND both maps in `BuildingBlocksSidebar/index.tsx` |
| SVG has `width`/`height` but no `viewBox` | Always use Simple Icons CDN SVG which includes `viewBox="0 0 24 24"` |
| Used provider-internal jargon ("Namespace") | Use provider's own UI terminology ("Workspace") |
| Forgot `iconSrc` in component/trigger mapper | Set `iconSrc: <name>Icon` in all mapper `props()` / `getTriggerProps()` returns |
| Didn't run `make gen.components.docs` | Always regenerate after config field changes |
| Renamed config field but left old key in tests | Update all `_test.go` fixtures to use the new key |
| No test assertion on HTTP request body fields | Add test to verify exact JSON field names and all required fields are present |
