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
- in creation of pr do not tag or refrence any other pr/iussue

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
