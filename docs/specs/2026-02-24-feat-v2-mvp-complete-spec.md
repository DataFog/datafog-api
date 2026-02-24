---
slug: 2026-02-24-feat-v2-mvp-complete
plan_mode: execution
spike_recommended: no
status: active
owner: sidmohan
created: 2026-02-24
---

# DataFog API v2 — Complete MVP

## Purpose

Close every gap between the current v2 branch (a working but incomplete policy gating prototype) and a shippable MVP that can detect PII at parity with datafog-python, enforce policy decisions with real data transformation, demonstrate end-to-end execution in a live demo, and integrate cleanly into Claude Code and OpenAI Codex via the existing adapter branches.

## Current State (Honest Baseline)

| Component | What works | What's missing |
|---|---|---|
| Scanner | 5 regex patterns, static confidence | No NER, no IP/date/zip, no PERSON/ORG, no Luhn, no multi-engine |
| Policy engine | Priority-sorted string-equality rules | No pattern matching, adapter registry not wired in |
| Transforms | mask/redact/tokenize/anonymize on API | Shim ignores transform plans — only does allow/deny |
| Shim | Real command gating via exec, calls /v1/decide | Doesn't apply redaction before allowing, events write-only |
| Receipts | JSONL to disk, survives restart | No querying beyond by-ID, no rotation |
| Demo | Static HTML calling API, "what-if" only | No real command execution, no shim integration |
| Platform adapters | v2-claude and v2-codex branches exist | Not merged, need base branch to be complete first |

## Scope

### In scope

**WS1 — PII Detection Parity**
- Expand entity types to 10+: email, phone, SSN, credit card, API key, IP address (IPv4), date (common formats), zip code (US 5-digit/ZIP+4), PERSON, ORGANIZATION/LOCATION
- Regex engine for structured entities with real validation (Luhn check for credit cards, IP range validation)
- Go-native NER engine for unstructured entities (PERSON, ORG, LOCATION) — dictionary/heuristic-based or lightweight model, no Python/cgo dependency
- Multi-engine cascade: regex (fast, always available) → NER (when enabled)
- Graceful degradation: NER unavailable → regex-only with warning
- Three anonymization strategies: redact (`[REDACTED]`), replace/pseudonymize (`[PERSON_A1B2C3]`), hash (SHA256)
- Selective entity filtering on all scan/transform endpoints
- Confidence scores: 1.0 for regex matches, configurable threshold for NER

**WS2 — Enforcement Gap Closure**
- Shim applies transform plans on `allow_with_redaction` decisions: stdin/file content redacted before command executes
- Wire adapter registry into policy rules (rules can match on canonical adapter names, not just raw strings)
- Events endpoint: `GET /v1/events` — query decision events with filters (time range, decision type, adapter)
- Disk-backed idempotency cache (survives restart)
- Receipt rotation/archival (configurable max size, auto-rotate)

**WS3 — Interactive Demo with Real Execution**
- Demo server: new Go HTTP handler (can be a mode of the existing binary or separate `cmd/datafog-demo`)
- `POST /demo/exec` — run a shell command through the shim, return decision + stdout/stderr
- `POST /demo/write-file` — write content through the shim to a sandbox, return decision + result
- `POST /demo/read-file` — read a file through the shim, return decision + content (or redacted content)
- Sandboxed temp directory for all demo file operations
- Updated `docs/demo.html` UI: command input, file operation panel, real pipeline visualization
- Shows: input → scan findings → policy decision → enforcement action → real output

**WS4 — Platform Integration Readiness**
- Ensure adapter registry includes `claude` and `codex` as recognized adapters with aliases
- Policy rules can target `tools: ["claude", "codex"]` and `adapters: ["claude", "codex"]`
- Base branch supports the hook installation pattern used by both v2-claude and v2-codex
- Validate that both setup scripts (`scripts/claude-datafog-setup.sh`, `scripts/codex-datafog-setup.sh`) work against the updated base
- Merge or rebase v2-claude and v2-codex onto the completed v2 branch

### Boundaries

- No Python dependencies or cgo — pure Go
- No ML model training or fine-tuning — NER is dictionary/heuristic or pre-built lookup
- No OCR or image-based PII detection (datafog-python has this but it's out of scope for Go API MVP)
- No distributed processing (Spark equivalent)
- No persistent database — JSONL/file-based storage is acceptable for MVP
- No authentication UI — API token via env var is sufficient
- No cloud deployment automation — local/Docker is sufficient
- Demo sandbox is ephemeral — no persistent demo state
- Platform adapter testing requires actual Claude/Codex binaries installed (can't be automated in CI without them)

## Requirements

| ID | Requirement | Priority | Workstream |
|----|-------------|----------|------------|
| R1 | Detect 10+ entity types: email, phone, SSN, credit card, API key, IP address, date, zip code, PERSON, ORGANIZATION | must | WS1 |
| R2 | Credit card detection includes Luhn validation to reduce false positives | must | WS1 |
| R3 | IP address detection validates range (0-255 per octet) | must | WS1 |
| R4 | Go-native NER for PERSON/ORG/LOCATION without Python or cgo | must | WS1 |
| R5 | Multi-engine cascade: regex first, NER second, configurable via env var | must | WS1 |
| R6 | Three anonymization strategies: redact, replace (pseudonymize with entity-typed tokens), hash (SHA256) | must | WS1 |
| R7 | Selective entity filtering: caller can specify which entity types to detect/transform | must | WS1 |
| R8 | Backward compatible: existing /v1/scan, /v1/decide, /v1/transform, /v1/anonymize contracts unchanged | must | WS1 |
| R9 | Shim applies transform plans on allow_with_redaction — content is actually redacted before command executes | must | WS2 |
| R10 | Adapter registry wired into policy: rules can match on canonical adapter names | must | WS2 |
| R11 | Events endpoint: GET /v1/events with time range and decision type filters | should | WS2 |
| R12 | Disk-backed idempotency cache survives process restart | should | WS2 |
| R13 | Receipt store supports rotation (configurable max entries or file size) | should | WS2 |
| R14 | Demo server exposes /demo/exec, /demo/write-file, /demo/read-file endpoints | must | WS3 |
| R15 | Demo operations run through the shim gate (real policy enforcement, not simulated) | must | WS3 |
| R16 | Demo file operations use a sandboxed temp directory (auto-cleaned) | must | WS3 |
| R17 | Demo UI shows full pipeline: input → findings → decision → enforcement → output | must | WS3 |
| R18 | Adapter registry includes claude and codex with correct aliases | must | WS4 |
| R19 | Policy rules support adapter-based matching for claude and codex | must | WS4 |
| R20 | v2-claude and v2-codex bootstrap scripts work against updated v2 base | must | WS4 |
| R21 | All existing tests continue to pass; new functionality has test coverage | must | All |

## Success Criteria

1. `go test ./...` passes with 0 failures on the completed branch.
2. Scanning text containing all 10+ entity types returns correct findings with appropriate confidence scores.
3. Running `datafog-shim shell git push` with PII in the working directory triggers `allow_with_redaction` and the PII is actually redacted in the output — not just flagged.
4. The demo UI at `docs/demo.html` can execute a real command, show the real decision, and display real stdout/blocked output.
5. `datafog-shim hooks install claude` and `datafog-shim hooks install codex` both succeed and create correct wrapper scripts.
6. The PR from v2 to dev passes CI (gofmt, go vet, go test, gosec).

## Constraints

- Go 1.22+ (module spec), CI uses 1.24+
- No external service dependencies (self-contained binary)
- API contract backward compatible (new fields allowed, existing fields stable)
- File size for demo.html under 50KB (was 30KB, allowing growth for new features)
- Demo server must not expose shell execution without explicit opt-in flag (security)

## Risks

| Risk | Impact | Mitigation |
|------|--------|------------|
| Go-native NER accuracy for PERSON/ORG may be significantly lower than spaCy/GLiNER | Medium | Start with dictionary + heuristic approach; document accuracy tradeoff; design for pluggable engine so ML can be added later |
| Shim transform-on-enforcement may break piped command workflows | High | Observe mode as default for first release; enforce mode opt-in |
| Demo shell execution endpoint is a security surface | High | Require explicit `--enable-demo` flag; sandbox all operations; never expose on non-localhost |
| Merging v2-claude/v2-codex may have conflicts with WS1-WS3 changes | Low | Merge adapter branches last after base is stable |

## Priority

**critical** — this is the MVP gate for the entire DataFog v2 product line. Both adapter branches are blocked on this.

## Initial Milestone Candidates

| ID | Milestone | Observable outcome | Risk |
|----|-----------|--------------------|------|
| M1 | Scanner parity | 10+ entity types detected, Luhn/IP validation, regex engine complete | Low |
| M2 | NER engine | PERSON/ORG/LOCATION detected via Go-native approach, cascade works | Medium — accuracy |
| M3 | Anonymization strategies | replace + hash modes added alongside existing redact/mask/tokenize/anonymize | Low |
| M4 | Shim enforcement | allow_with_redaction actually redacts, transform plans applied | Medium — piping |
| M5 | Policy + adapter wiring | Adapter registry in rules, claude/codex adapters registered | Low |
| M6 | Events + persistence | Events endpoint, disk-backed idempotency, receipt rotation | Low |
| M7 | Demo server + UI | Real execution demo with command/file panels, sandboxed ops | Medium — security |
| M8 | Platform integration | v2-claude and v2-codex merged/rebased, setup scripts validated | Low |
| M9 | PR + CI | All tests pass, PR to dev created, CI green | Low |

## Key Decisions

1. **Go-native NER over cgo/Python**: Accepting lower accuracy for PERSON/ORG in exchange for single-binary deployment and zero external dependencies. Can be upgraded later with a pluggable engine interface.
2. **Demo requires explicit opt-in**: `--enable-demo` flag prevents accidental shell execution exposure. Demo endpoints only bind on localhost.
3. **Workstream sequencing**: WS1 (scanner) → WS2 (enforcement) → WS3 (demo) → WS4 (adapters). Each builds on the previous.

## Reference Artifacts

- DataFog Python repo: https://github.com/DataFog/datafog-python (v4.3.0, PII detection baseline)
- Existing API contract: `docs/contracts/datafog-api-contract.md`
- v2-claude branch: `origin/codex/v2-claude`
- v2-codex branch: `origin/codex/v2-codex`
- Current demo UI: `docs/demo.html`
- Honest v2 audit: conversation context (2026-02-24)

## Handoff

After approval, proceed directly to `he-plan` then `he-implement`. No spike needed — all unknowns are resolvable during planning. The implementer has full autonomy to commit, push, and create a PR to dev.

## Revision Notes

- v1: Initial spec from comprehensive audit of v2 state, datafog-python research, and adapter branch analysis.
