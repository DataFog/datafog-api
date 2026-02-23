---
title: "Update AGENTS.md"
use_when: "Creating or updating a project's AGENTS.md (agent instructions, conventions, and workflows)."
called_from:
  - he-bootstrap
  - he-learn
  - he-doc-gardening
---

# Update AGENTS.md

This runbook is repo-specific and **additive only**. It must not waive or override any gates enforced by skills.

AGENTS.md is the agent-facing README: a predictable place to put the few repo-specific instructions an agent needs to work effectively.

When `he-bootstrap` runs in a repo that already has `AGENTS.md`, it appends a managed block once using:

- `<!-- he-bootstrap:start -->`
- `<!-- he-bootstrap:end -->`

Do not edit outside your repo's intended scope when touching this managed block.

## What To Optimize For

- Keep it short and stable (a map, not an encyclopedia).
- Put only high-leverage, repo-specific guidance here: build/test commands, conventions, and hard constraints.
- Add rules over time when you observe repeated failure modes; do not try to predict everything up front.

## What To Put Elsewhere

- Long procedures, checklists, and evolving processes: `docs/runbooks/<topic>.md` and link from AGENTS.md.
- One-off migrations or multi-hour work: a plan/spec doc under `docs/` (not in AGENTS.md).

## Minimum Sections (Good Starting Point)

- Setup commands (install, dev, test, lint) in copy-pastable form.
- Repo map (where the important stuff lives; key entrypoints).
- Conventions (formatting, naming, dependency rules, boundaries).
- Safety and verification (what not to do; how to prove the change works here).
- Runbook index (links into `docs/runbooks/` for process).

## Rules Of Thumb When Editing AGENTS.md

- If it changes often, it probably belongs in a runbook, not AGENTS.md.
- Prefer "When X, do Y" over vague guidance.
- Make requirements verifiable (a command, a file path, an expected output).
- Avoid duplicating information already in `docs/`; link instead.
- Keep any `he-bootstrap` managed block concise and link-first to avoid disrupting existing user conventions.

## Quick Update Checklist

1. Confirm scope: are you editing the right AGENTS.md for the files you are touching (root vs nested)?
2. Keep it minimal: can you replace paragraphs with a link to a runbook?
3. Verify paths/commands exist:

```sh
rg -n "docs/runbooks|PLANS\\.md|Runbooks|Setup|test|lint" AGENTS.md
find docs/runbooks -type f -maxdepth 2 -name "*.md" -print
```
