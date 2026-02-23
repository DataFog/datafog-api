---
title: "Security"
use_when: "Capturing security expectations for this repo: threat model, auth/authorization, data sensitivity, compliance, and required controls."
---

## Threat Model
- Identify assets (data, credentials, money), actors (users, admins, services), and trust boundaries.
- Assume untrusted input everywhere; document the highest-risk entry points (HTTP, CLI args, webhooks, uploads).
- Call out "must not happen" failures (auth bypass, data exfiltration, privilege escalation).

## Auth Model
- Default-deny authorization and least privilege; make role/permission checks explicit.
- Separate authentication (who) from authorization (what they can do).
- Prefer centralized enforcement (middleware/policy layer) over scattered checks.

## Data Sensitivity
- Classify data (public, internal, confidential, secret) and list the sensitive fields.
- Never log secrets or credentials; treat tokens, passwords, API keys as secrets.
- Encrypt in transit; document at-rest encryption expectations if storing sensitive data.

## Compliance
- State explicitly whether regulated data is in scope; if unknown, assume it is not until confirmed.
- If handling PII, document retention and deletion expectations and who can access it.

## Controls
- Secrets management: no secrets in git; rotate on leak; minimal scopes.
- Dependency hygiene: lockfiles, update cadence, and vulnerability scanning expectations.
- Input validation and output encoding at boundaries; protect against injection.
