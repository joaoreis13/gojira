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

1. Check state before doing anything: `gojira profile list`,
   `gojira auth status --profile <alias>`. Exactly one profile is active at
   a time (marked in `profile list`); most commands default to it, and
   switching (`gojira profile use <alias>`) is always a manual, explicit
   step — never switch it on the user's behalf without asking.
2. **Setting up a new profile is not something you can do unattended.** It
   needs a Client ID and Secret from an Atlassian OAuth 2.0 (3LO) app that
   only the user can create, at
   https://developer.atlassian.com/console/myapps/ (steps in README.md's
   "One-time setup" section). Once the user gives you those values:
   ```bash
   gojira profile add <alias> --base-url https://<team>.atlassian.net \
     --client-id <id> --client-secret <secret>
   gojira profile use <alias>         # activates it, forcing a token refresh
   gojira auth login --profile <alias>   # opens a browser; the user completes this
   ```
   `profile add` never auto-activates, even for the very first profile —
   activation is always its own explicit step (`--activate` on `add`, or a
   separate `profile use`).
3. Make calls with the generic passthrough, which reaches every Jira REST
   API v3 endpoint including admin/destructive ones:
   ```bash
   gojira api <METHOD> <path> [--data '<json>'|@file|-] [--query k=v] \
     [--fields a,b.c] [--output json|text] [--yes]
   ```
   `<path>` is relative to `/rest/api/3` (e.g. `/issue/PROJ-1`,
   `/search/jql`, `/project`). Exception: attachment *uploads*
   (`POST /issue/{key}/attachments`) aren't reachable — gojira always sends
   `Content-Type: application/json` and has no multipart support; tell the
   user to use the Jira web UI instead. Downloading attachment content
   (`GET /attachment/content/{id} > file`) works fine.
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
   `gojira search '<JQL>' --output text`, `gojira checklist <subcommand>` —
   but the generic `api` command is the source of truth for anything else;
   full flags via `--help` or
   [`internal/skillassets/gojira/references/commands.md`](internal/skillassets/gojira/references/commands.md).

## Where to look up operation-specific details

Don't guess request/response field names. Look them up:

- Full REST API v3 reference, browsable by resource group, each operation
  listing its exact path, request/response schema, and required OAuth
  scope: https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/
- JQL syntax and field names: https://support.atlassian.com/jira-software-cloud/docs/jql-fields/
- OAuth scopes (for `gojira profile add --scopes`):
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

## graphify

This project has a knowledge graph at graphify-out/ with god nodes, community structure, and cross-file relationships.

When the user types `/graphify`, use the installed graphify skill or instructions before doing anything else.

Rules:
- When graphify-out/graph.json exists, run `graphify query "<question>"` before your first grep, search or source-file read for a codebase question, and put this rule in every subagent prompt that explores code. Read raw files only for lines the graph or an earlier read already located. Use `graphify path "<A>" "<B>"` for relationships and `graphify explain "<concept>"` for focused concepts. These return a scoped subgraph, usually much smaller than GRAPH_REPORT.md or raw grep output.
- Dirty graphify-out/ files are expected after hooks or incremental updates; dirty graph files are not a reason to skip graphify. Only skip graphify if the task is about stale or incorrect graph output, or the user explicitly says not to use it.
- If graphify-out/wiki/index.md exists, use it for broad navigation instead of raw source browsing.
- Read graphify-out/GRAPH_REPORT.md only for broad architecture review or when query/path/explain do not surface enough context.
- After modifying code, run `graphify update .` to keep the graph current (AST-only, no API cost).
- A committed graphify-out/graph.json is rewritten whole on every build, so a large diff to it in a pull request is expected. Do not flag it as churn or suggest skipping the graph rebuild or its hooks.
