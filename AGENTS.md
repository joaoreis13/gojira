# AGENTS.md

Instructions for AI coding agents working with this repository or asked to
use the `gojira` CLI it builds. (If you're an agent that discovers skills
from `~/.claude/skills/` or `.claude/skills/`, prefer running
`gojira skill install` once — see below — over re-reading this file every
time; the installed skill covers the same ground.)

## What this is

`gojira` is a single static Go binary that calls the Jira Cloud REST API v3
over OAuth 2.0 (3LO), with automatic token refresh. It's meant to be driven
by both humans and agents from the command line — see [README.md](README.md)
for the full human-facing walkthrough. This file is the condensed,
operational version for an agent.

## Installing

```bash
go install github.com/joaoreis13/gojira/cmd/gojira@latest
```

Requires a Go toolchain. If one isn't available, tell the user rather than
installing Go or downloading a release binary on your own — that crosses
into "explicit permission needed" territory, not routine setup.

Verify: `command -v gojira`.

## Giving yourself the on-machine skill (recommended, one-time)

```bash
gojira skill install            # personal, all projects: ~/.claude/skills/gojira
gojira skill install --project  # this project only: ./.claude/skills/gojira
```

This copies a `SKILL.md` (+ a static command reference) that a Claude
Code-compatible harness will surface automatically in future sessions, so
the operational guidance below doesn't need to be re-derived from this file
each time.

## Using gojira to talk to Jira

1. Check state before doing anything: `gojira site list`,
   `gojira auth status --site <alias>`.
2. **Setting up a new site is not something you can do unattended.** It
   needs a Client ID and Secret from an Atlassian OAuth 2.0 (3LO) app that
   only the user can create, at
   https://developer.atlassian.com/console/myapps/ (steps in README.md's
   "One-time setup" section). Once the user gives you those values:
   ```bash
   gojira site add <alias> --base-url https://<team>.atlassian.net \
     --client-id <id> --client-secret <secret> --default
   gojira auth login --site <alias>   # opens a browser; the user completes this
   ```
3. Make calls with the generic passthrough, which reaches every Jira REST
   API v3 endpoint including admin/destructive ones:
   ```bash
   gojira api <METHOD> <path> [--data '<json>'|@file|-] [--query k=v] \
     [--fields a,b.c] [--output json|text] [--yes]
   ```
   `<path>` is relative to `/rest/api/3` (e.g. `/issue/PROJ-1`,
   `/search/jql`, `/project`).
4. Prefer `--fields`/`--output text` whenever you don't need a full object —
   it keeps responses cheap in your own context window. This is the same
   reasoning behind gojira's `--fields`/`--output` design in the first
   place (see README.md's "Why not just use..." section).
5. `DELETE` calls require `--yes` or they'll block on an interactive prompt
   that will never come in a non-interactive/agent context. **Only pass
   `--yes` after the user has actually confirmed the destructive action —
   never preemptively.** Treat any bulk operation or admin/configuration
   change (permission schemes, webhooks, workflows, project settings) with
   the same caution even outside `DELETE`.
6. Convenience wrappers exist for common cases — `gojira whoami`,
   `gojira search '<JQL>' --output text` — but the generic `api` command is
   the source of truth for anything else; full flags via `--help` or
   [`internal/skillassets/gojira/references/commands.md`](internal/skillassets/gojira/references/commands.md).

## Where to look up operation-specific details

Don't guess request/response field names. Look them up:

- Full REST API v3 reference, browsable by resource group, each operation
  listing its exact path, request/response schema, and required OAuth
  scope: https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/
- JQL syntax and field names: https://support.atlassian.com/jira-software-cloud/docs/jql-fields/
- OAuth scopes (for `gojira site add --scopes`):
  https://developer.atlassian.com/cloud/jira/platform/scopes-for-oauth-2-3LO-and-forge-apps/

## Working on gojira's own source

- `go build ./... && go vet ./... && gofmt -l . && go test ./...` before
  considering any change done — this is exactly what CI
  ([.github/workflows/ci.yml](.github/workflows/ci.yml)) runs.
- The `api` passthrough (`internal/cli/api_cmd.go`) is the load-bearing
  piece: it's what gives gojira full REST API coverage without hand-built
  wrappers per endpoint. Keep new convenience commands (`internal/cli/*_cmd.go`)
  thin wrappers over `internal/client.Client.Do`, not reimplementations.
- Skill content lives in `internal/skillassets/gojira/` and is embedded into
  the binary via `go:embed` — update it in lockstep with real CLI behavior
  changes (flags, endpoints, defaults), the same way you'd update
  README.md.

## Guardrails

- Never handle the user's Atlassian account password — gojira never asks
  for one; only an OAuth app's Client ID/Secret and a one-time browser
  login.
- Never invent or reuse Client ID/Secret values from elsewhere — they must
  come from the user's own Atlassian Developer Console app for this tool.
- Don't commit real client secrets, tokens, or `~/.config/gojira` /
  `~/Library/Application Support/gojira` contents anywhere in this repo.
