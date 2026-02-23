# Generated Context

This directory holds generated reference context that agents create to reason about the codebase. Files here are auto-generated snapshots — not hand-authored documentation.

## When to Create

Create a generated context file when a skill (`he-implement`, `he-review`, `he-doc-gardening`) discovers relevant project infrastructure during its workflow. Discovery signals and corresponding context files:

| Discovery Signal | Context to Create | Example Filename |
|---|---|---|
| Database migrations or schema files exist | Schema snapshot | `db-schema.md` |
| Route definitions or API framework detected | API endpoint index | `api-schema.md` |
| UI component hierarchy (React, Vue, etc.) | Component tree map | `component-tree.md` |
| Complex module dependency structure | Dependency graph | `dependency-graph.md` |

This is not exhaustive — create whatever context helps agents reason about the project. The key rule: only create files for infrastructure that actually exists.

## Format Contract

Every generated file must include:

```
- last_updated: YYYY-MM-DD HH:MM
```

The `he-docs-lint` CI gate checks this timestamp on all files in this directory (except README.md and memory.md).

## Rules

- **Do not** create files for infrastructure the project does not have.
- **Do not** manually edit generated files — regenerate them from source.
- **Do** regenerate when the underlying source changes (migrations added, routes modified, etc.).

## memory.md

`memory.md` is a separate concept: it is a scratchpad for observations and patterns discovered during work, processed by `he-learn`. It is not auto-generated context and is not subject to the `last_updated` requirement.
