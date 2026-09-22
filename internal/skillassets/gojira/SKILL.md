---
name: gojira
description: Use when the user wants to read, search, create, update, transition, comment on, or delete Jira Cloud issues, or manage projects/workflows/webhooks/admin settings — or otherwise wants an AI agent to call the Jira REST API. Also use when installing, authenticating, or troubleshooting the gojira CLI itself.
---

# gojira — Jira Cloud CLI for agents

gojira is a single static binary that calls the Jira Cloud REST API v3 over
OAuth 2.0, with automatic token refresh. Source and full docs:
https://github.com/joaoreis13/gojira

## First: check it's installed and authenticated

```
command -v gojira || echo "not installed"
gojira site list
gojira auth status --site <alias>
```

- **Not installed** — `go install github.com/joaoreis13/gojira/cmd/gojira@latest`
  (requires Go). If Go isn't available, tell the user; don't install a Go
  toolchain or download a release binary without asking first.
- **No site configured, or `site list` doesn't show the one you need** —
  this requires the user's own Atlassian OAuth app (Client ID + Secret from
  https://developer.atlassian.com/console/myapps/). You cannot create this
  for them. Point them at the repo README's "One-time setup" section, then
  run `gojira site add <alias> --base-url <url> --client-id <id> --client-secret <secret>`
  once they give you the values.
- **Not logged in / token status unclear** — `gojira auth login --site <alias>`.
  This opens a browser for the user to approve; it's an interactive step —
  tell the user to complete it, don't try to script around it.

## Making calls

For anything without a dedicated command below, use the generic passthrough
— it reaches every Jira REST API v3 endpoint, including admin/destructive
ones:

```
gojira api <METHOD> <path> [--data '<json>' | --data @file | --data -] \
  [--query key=value ...] [--fields a,b.c] [--output json|text] [--yes]
```

- `<path>` is relative to `/rest/api/3` (e.g. `/issue/PROJ-1`, `/project`,
  `/search/jql`). Look up the exact request body, response shape, and
  required OAuth scope for a specific endpoint before constructing `--data`
  — see "Where to find operation details" below. Don't guess field names.
- **Always pass `--fields` and/or `--output text`** when you don't need the
  full object — this is what keeps Jira's (often large) JSON responses
  cheap in your context window. Full JSON is only the default because it's
  the safe choice for scripting, not because you should consume it that way.
- `DELETE` and other calls gojira treats as destructive require `--yes`, or
  they block waiting on an interactive stdin prompt that won't come from an
  agent. Only pass `--yes` after the user has actually confirmed the
  destructive action — treat it the same as any other irreversible action
  you'd normally ask permission for first.
- **Exception to "every endpoint":** attachment uploads
  (`POST /issue/{key}/attachments`) don't work — gojira always sends JSON
  and has no multipart support. Tell the user to upload via the Jira web UI.
  Downloading attachment content (`GET /attachment/content/{id} > file`) is
  fine.

## Convenience commands (thin wrappers, prefer these when they fit)

- `gojira whoami` — the authenticated user (`GET /myself`).
- `gojira search '<JQL>' --output text [--fields ...] [--jira-fields ...]`
  — JQL search (`POST /search/jql`). Paginated by token, not offset: if the
  response's `isLast` is `false`, pass its `nextPageToken` back via
  `--page-token` for the next page. If a literal value in your JQL (a
  project key, custom field value, etc.) happens to be a JQL reserved word
  (`IN`, `AND`, `OR`, `EMPTY`, ...), Jira rejects it with "Expecting either
  a value, list or function but got '<WORD>'" — quote it: `project = "IN"`.

Full flags for every command: `gojira <command> --help`, or the static copy
in `references/commands.md` next to this file.

## Where to find operation details

- Full REST API v3 reference, browsable by resource group (Issues,
  Projects, Workflows, Webhooks, Permissions, ...), each with exact paths,
  request/response fields, and required scopes:
  https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/
- JQL syntax/fields: https://support.atlassian.com/jira-software-cloud/docs/jql-fields/
- OAuth scopes reference (for `gojira site add --scopes`):
  https://developer.atlassian.com/cloud/jira/platform/scopes-for-oauth-2-3LO-and-forge-apps/

## Guardrails

- Never type or accept the user's Atlassian account password anywhere —
  gojira never asks for one; only OAuth app Client ID/Secret and a browser
  login.
- Never invent Client ID/Secret values — they must come from the user's own
  Atlassian Developer Console app.
- Treat `DELETE`, bulk operations, and admin/configuration-changing calls
  (permission schemes, webhooks, workflows, project settings) as
  destructive: confirm with the user before passing `--yes`.
