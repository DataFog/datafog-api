---
slug: 2026-02-24-feat-interactive-demo-ui
plan_mode: lightweight
spike_recommended: no
status: active
owner: sidmohan
created: 2026-02-24
---

# Interactive Demo UI

## Purpose

Provide a single-page interactive playground that lets anyone with the server running on localhost see the DataFog API in action — scan PII, view policy decisions, and compare all four transform modes — without leaving the browser or reading docs first.

## Scope

### In scope

- Single self-contained HTML file (`docs/demo.html`), no build tools or dependencies beyond a CDN-hosted minimal CSS (or inline styles)
- Pre-populated sample text containing all 5 entity types (email, phone, SSN, API key, credit card)
- Editable text input so users can paste their own content
- **Scan panel**: call `POST /v1/scan`, display detected entities with type badges, confidence scores, and highlighted positions in the source text
- **Decide panel**: configurable action type/tool/command fields, call `POST /v1/decide`, display decision (allow / deny / transform / allow_with_redaction), matched rules, and transform plan
- **Transform panel**: toggle between all 4 modes (mask, tokenize, anonymize, redact), call `POST /v1/transform`, show transformed output with a visual diff against the original
- One-click "Run All" button that executes the full pipeline (scan → decide → transform) sequentially and populates all panels
- Connection status indicator (hits `GET /health` on load, shows server version and policy info)
- Works against `http://localhost:8080` (server must already have CORS support — already shipped)

### Boundaries

- No authentication UI — demo assumes no `DATAFOG_API_TOKEN` is set (local dev default)
- No receipt viewer — receipts are an audit concern, not a demo concern
- No server-side changes — the HTML file is purely a client; the API is unchanged
- No build step, no npm, no bundler — vanilla HTML/CSS/JS only
- No persistent state — page reload resets everything
- Mobile responsiveness is nice-to-have, not required

## Requirements

| ID | Requirement | Priority |
|----|-------------|----------|
| R1 | Page loads with sample text pre-filled containing at least one of each entity type (email, phone, SSN, API key, credit card) | must |
| R2 | User can edit the text freely before running any operation | must |
| R3 | "Scan" button calls `POST /v1/scan` and renders findings as highlighted spans in the text with entity type labels and confidence | must |
| R4 | "Decide" button calls `POST /v1/decide` with user-configurable action fields (type, tool, command) and displays the decision, matched rules, reason, and any transform plan | must |
| R5 | "Transform" section lets the user pick a mode (mask / tokenize / anonymize / redact) or per-entity modes, calls `POST /v1/transform`, and shows the transformed output | must |
| R6 | "Run All" button executes scan → decide → transform in sequence, populating all panels with a single click | must |
| R7 | Each API call shows a loading state and elapsed time | should |
| R8 | Errors from the API are displayed inline with the error code and message | must |
| R9 | On page load, `GET /health` is called; connection status + policy version shown in a header bar | should |
| R10 | Visual diff between original text and transformed output (e.g., side-by-side or inline highlights showing what changed) | should |

## Success Criteria

1. A new user can open `docs/demo.html` in a browser, click "Run All", and see scan results, a policy decision, and transformed text within 3 seconds (given a running server).
2. All 5 entity types are visually distinguishable in the scan results.
3. Switching transform modes re-runs the transform and updates the output without re-scanning.
4. The file is a single `demo.html` with zero external dependencies beyond optional CDN CSS — works offline if the CDN is cached.
5. `go test ./...` continues to pass (no server-side changes).

## Constraints

- Must work on Chrome, Edge, Firefox (current versions)
- No server-side changes — client-only HTML file
- File size should stay under 30 KB to keep it easy to review in a PR
- Must handle server-down gracefully (show connection error, don't break the page)

## Priority

**high** — this is the first thing someone sees when evaluating the project locally.

## Initial Milestone Candidates

| ID | Milestone | Observable outcome | Risk |
|----|-----------|--------------------|------|
| M1 | Page skeleton + health check | HTML file loads, shows server connection status and policy version in header | Low |
| M2 | Scan panel | Editable text area, "Scan" button, findings rendered with highlights and badges | Low |
| M3 | Decide panel | Action fields (type/tool/command), "Decide" button, decision + matched rules displayed | Low |
| M4 | Transform panel | Mode selector, "Transform" button, output with visual diff | Medium — diff rendering |
| M5 | Run All pipeline + polish | Sequential execution, loading states, timing, error handling, final styling pass | Low |

## Handoff

After approval, proceed to `he-plan` for implementation planning. No spike needed — the API contract is stable, CORS is already in place, and the scope is well-bounded.

## Revision Notes

- v1: Initial spec from interactive session.
