# gojira

An OAuth-managed CLI for the **Jira Cloud REST API** that reaches every
endpoint the API exposes — including admin and destructive operations —
through a generic passthrough command, with automatic OAuth 2.0 token
refresh and output shaped for cheap consumption by scripts or AI agents.

It ships as a single static binary: no runtime to install, works the same
on macOS, Linux, and Windows, and isn't tied to any particular AI agent or
tool — anything that can run a subprocess and read stdout can drive it.

## Why not just use the Jira MCP / an existing Jira CLI?

- **Full REST API surface, day one.** `gojira api <METHOD> <path>` hits any
  `/rest/api/3/...` endpoint directly. New Jira API surface doesn't require
  a gojira release — the escape hatch already covers it.
- **OAuth 2.0 (3LO) with managed refresh.** Log in once in a browser; gojira
  stores tokens in your OS's native credential store and transparently
  refreshes them, instead of you managing long-lived API tokens by hand.
- **Output built for token-cheap consumption.** `--fields` narrows JSON
  responses to just the paths you asked for, and `--output text` renders a
  line-per-item summary instead of dumping full Jira objects.
- **Destructive calls are guarded by default.** Any `DELETE` prompts for
  confirmation unless you pass `--yes` — including when driven by an agent.

## Install

```bash
go install github.com/joaoreis13/gojira/cmd/gojira@latest
```

This puts the `gojira` binary in `$(go env GOBIN)`, or `$(go env GOPATH)/bin`
if `GOBIN` isn't set — make sure that directory is on your `PATH`.

Building from a local clone works the same way:

```bash
git clone https://github.com/joaoreis13/gojira.git
cd gojira
go install ./cmd/gojira
```

(Prebuilt binaries for macOS/Linux/Windows/amd64/arm64 are published on the
[Releases](https://github.com/joaoreis13/gojira/releases) page for each
tagged version, built by the [.goreleaser.yaml](.goreleaser.yaml) config.)

## One-time setup: create an Atlassian OAuth app

Jira Cloud OAuth 2.0 (3LO) apps are registered per-developer, not shared —
you need your own. Full guide:
[OAuth 2.0 (3LO) apps](https://developer.atlassian.com/cloud/jira/platform/oauth-2-3lo-apps/).

1. Go to <https://developer.atlassian.com/console/myapps/>, select **Create**
   → **OAuth 2.0 integration**, and name it.
2. Select **Permissions** in the left menu and add the Jira Cloud REST API.
   Then select **Configure** → **Add** for each scope you want. A broad
   starting set (see the full
   [scopes reference](https://developer.atlassian.com/cloud/jira/platform/scopes-for-oauth-2-3LO-and-forge-apps/)
   for everything available):
   - Classic scopes (recommended by Atlassian over the granular ones below):
     `read:jira-work`, `write:jira-work`, `read:jira-user`,
     `manage:jira-project`, `manage:jira-configuration`,
     `manage:jira-webhook`.
   - `offline_access` — not Jira-specific, but required to receive a
     *refresh* token instead of only a short-lived access token.
3. Select **Authorization** in the left menu → **Configure** next to
   "OAuth 2.0 (3LO)" → set the callback URL to
   `http://localhost:51837/callback` (or whatever port you'll pass as
   `--redirect-port`) → **Save changes**.
4. Select **Settings** in the left menu and copy the **Client ID** and
   **Secret**. Both are required: Atlassian's 3LO apps are confidential
   clients only — as of this writing they do **not** support PKCE/public
   clients for apps created in the developer console (confirmed by an
   Atlassian staff member in
   [this thread](https://community.developer.atlassian.com/t/oauth-2-0-with-proof-key-for-code-exchange-pkce/80173)),
   so there's no way to avoid holding a client secret.

## Configure and log in

```bash
gojira site add work \
  --base-url https://yourteam.atlassian.net \
  --client-id <your-client-id> \
  --client-secret <your-client-secret> \
  --default

gojira auth login --site work   # opens your browser
```

`auth login` runs the OAuth 2.0 (3LO) authorization-code flow (a local,
short-lived HTTP listener on `--redirect-port` catches the callback),
resolves and caches the Jira Cloud id for `work` via
[`GET /oauth/token/accessible-resources`](https://developer.atlassian.com/cloud/jira/platform/oauth-2-3lo-apps/#3-1-get-the-cloudid-for-your-site),
and stores tokens in your OS keyring (macOS Keychain / Windows Credential
Manager / Linux Secret Service), falling back to a `0600` file under the
gojira config directory if no OS keyring is available.

## Usage

```bash
# Anything the REST API exposes, via the generic passthrough:
gojira api GET /issue/PROJ-123
gojira api POST /search/jql --data '{"jql":"project = PROJ","fields":["summary","status"]}' \
  --fields issues.key,issues.fields.summary --output text
gojira api POST /issue --data '{
  "fields": {"project": {"key": "PROJ"}, "summary": "New issue", "issuetype": {"name": "Task"}}
}'
gojira api DELETE /issue/PROJ-999 --yes   # destructive calls require --yes or an interactive "yes"

# Convenience commands built on the same client:
gojira whoami
gojira search 'assignee = currentUser() AND status != Done' --output text

# Multiple sites:
gojira site add personal --base-url https://mysite.atlassian.net --client-id ... --client-secret ...
gojira api GET /myself --site personal

# Token lifecycle:
gojira auth status
gojira auth refresh
gojira auth logout   # deletes local credentials only; revoke the grant at
                      # https://id.atlassian.com/manage-profile/apps if needed
```

Exact request/response fields and required scopes for the calls above:
[`GET /issue/{issueIdOrKey}`](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/#api-rest-api-3-issue-issueidorkey-get) ·
[`POST /search/jql`](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-search/#api-rest-api-3-search-jql-post) ·
[`POST /issue`](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/#api-rest-api-3-issue-post) ·
[`DELETE /issue/{issueIdOrKey}`](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issues/#api-rest-api-3-issue-issueidorkey-delete) ·
[`GET /myself`](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-myself/#api-rest-api-3-myself-get).

Every other Jira REST API v3 operation, including ones gojira has no
dedicated command for, is reachable the same way through
`gojira api <METHOD> <path>` — browse the full
[REST API v3 reference](https://developer.atlassian.com/cloud/jira/platform/rest/v3/intro/)
by resource group (Issues, Projects, Workflows, Webhooks, Permissions, ...)
for any other operation's exact path, request body, and required scopes.

## Command reference

Every command also documents itself via `--help`; this is the shape of it:

| Command | Purpose |
|---|---|
| `gojira site add <alias> --base-url ... --client-id ... --client-secret ...` | Register a Jira site's OAuth app config. `--scopes`, `--redirect-port`, `--default` are optional. |
| `gojira site list` / `site use <alias>` / `site remove <alias>` | List, switch default, or remove a configured site. |
| `gojira auth login [--site] [--no-browser]` | Run the browser OAuth flow and store tokens. `--no-browser` prints the URL instead of auto-opening it. |
| `gojira auth status [--site]` | Show token expiry and whether a refresh token is stored. |
| `gojira auth refresh [--site]` | Force an immediate token refresh. |
| `gojira auth logout [--site]` | Delete locally stored credentials (does not revoke the grant on Atlassian's side). |
| `gojira api <METHOD> <path> [--data] [--query] [--fields] [--output] [--pretty] [--yes] [--site]` | Call any REST API v3 endpoint. `<path>` is relative to `/rest/api/3` (e.g. `/issue/PROJ-1`). |
| `gojira whoami [--site]` | `GET /myself` — the authenticated user. |
| `gojira search <JQL> [--max-results] [--page-token] [--jira-fields] [--fields] [--output] [--site]` | `POST /search/jql` — run a JQL query. |
| `gojira skill install [--project] [--dest <dir>]` | Install the bundled Agent Skill (see below). |

### `--data` (on `api`)

- Inline JSON: `--data '{"key":"value"}'`
- From a file: `--data @body.json`
- From stdin: `--data -`

### Output flags (on `api` and `search`)

- `--fields a,b.c` — keep only these dot-paths from the response. Applied
  per-item when the response is a list or wraps one in an `issues`/`values`
  array (e.g. `issues.key,issues.fields.summary`).
- `--output json|text` — `text` renders one line per item for recognized
  shapes (issue search results, a single issue, `values`-paginated admin
  lists); anything else falls back to compact JSON either way.
- `--pretty` — indent JSON output (only relevant with `--output json`).

## For AI agents

gojira is designed to be driven by an AI agent as readily as by a human —
that's what `--fields`/`--output text` and the destructive-action `--yes`
gate are for.

- **[AGENTS.md](AGENTS.md)** — condensed operational guide for any agent
  reading this repository directly: install/setup/usage, where to look up
  endpoint-specific request/response shapes, and guardrails around
  destructive calls and credentials.
- **`gojira skill install`** — installs a portable Agent Skill (`SKILL.md` +
  a static command reference) onto this machine, so a Claude Code-compatible
  harness picks up the same guidance automatically in future sessions
  without re-reading this README. Installs to `~/.claude/skills/gojira` by
  default; `--project` installs to `./.claude/skills/gojira` instead, and
  `--dest <dir>` targets any other location. Source:
  [`internal/skillassets/gojira/`](internal/skillassets/gojira/).

## Troubleshooting

- **`exchange authorization code: ... invalid_client` or similar during
  `auth login`** — double check `--client-id`/`--client-secret` match the
  app's Settings page exactly, and that the callback URL configured on the
  app (Authorization page) is exactly `http://localhost:<redirect-port>/callback`.
- **`listen on 127.0.0.1:<port> ...: address already in use`** — another
  process is using that port; pick a different `--redirect-port` on both
  `site add` and the app's callback URL configuration, then re-run
  `auth login`.
- **`invalid_grant` on refresh** — refresh tokens rotate and expire after 90
  days of inactivity (or if your Atlassian password changed); run
  `gojira auth login` again. Details in the
  [OAuth 2.0 (3LO) FAQ](https://developer.atlassian.com/cloud/jira/platform/oauth-2-3lo-apps/#how-do-i-get-a-new-access-token--if-my-access-token-expires-or-is-revoked-).
- **`no accessible site matches base_url ...`** — the authorizing Atlassian
  account doesn't have access to that site, or `--base-url` doesn't exactly
  match the site's URL (no trailing slash needed either way; matching is
  case-insensitive).
- **429 responses** — gojira retries 429/5xx up to 3 times, honoring
  `Retry-After` when Jira sends it. Sustained rate limiting is documented at
  [Rate limiting](https://developer.atlassian.com/cloud/jira/platform/rate-limiting/).

## Uninstall

```bash
rm "$(go env GOPATH)/bin/gojira"   # or wherever `which gojira`/`where gojira` points
```

Config and site data (OAuth app info, cached cloud ids, and the file-based
credential fallback if your OS keyring wasn't available) live under
`os.UserConfigDir()`'s `gojira` subdirectory — remove it to reset
everything:

- Linux: `rm -rf ~/.config/gojira`
- macOS: `rm -rf ~/Library/Application\ Support/gojira`
- Windows: `rmdir /s "%AppData%\gojira"`

If you logged in and your OS keyring was available, also remove gojira's
entries from it (e.g. Keychain Access on macOS, Credential Manager on
Windows, Secret Service/`secret-tool` on Linux) — `gojira auth logout`
before uninstalling does this for you per site.

If you ran `gojira skill install`, also remove `~/.claude/skills/gojira`
(or `./.claude/skills/gojira`, or your `--dest` path).

## Security notes

- Tokens live in your OS credential store by default, never on disk in
  plaintext unless no keyring is available on the machine (then a `0600`
  file is used, under the config directory above).
- `gojira site add` only ever needs your own OAuth app's client id and
  secret — no Jira account password or long-lived personal API token is
  ever entered into the tool.
- Grant only the scopes you need; `gojira site add --scopes ...` overrides
  the broad default set. See the
  [scopes reference](https://developer.atlassian.com/cloud/jira/platform/scopes-for-oauth-2-3LO-and-forge-apps/)
  for the full list and what each one unlocks.
- This "one OAuth app per user" pattern is meant for personal/individual
  use of your own Jira account, not for distributing an app to other
  people — Atlassian's guidance for that case is different (a single,
  shared, Marketplace-reviewed app); see the "Overview" section of the
  [OAuth 2.0 (3LO) apps guide](https://developer.atlassian.com/cloud/jira/platform/oauth-2-3lo-apps/).

## Status

Early stage. The `api` passthrough is the primary, fully-general interface;
convenience commands (`whoami`, `search`) exist to demonstrate the pattern
and will grow incrementally. Cloud only — Jira Server/Data Center (which has
no OAuth 2.0 3LO) is out of scope for now.

**Known limitation: attachment uploads.** `gojira api` always sends
`Content-Type: application/json` and has no `multipart/form-data` support, so
[`POST /issue/{issueIdOrKey}/attachments`](https://developer.atlassian.com/cloud/jira/platform/rest/v3/api-group-issue-attachments/#api-rest-api-3-issue-issueidorkey-attachments-post)
(which requires a multipart body and an `X-Atlassian-Token: no-check` header)
can't be reached through gojira. Use the Jira web UI to upload attachments.
Downloading attachment *content* works fine —
`gojira api GET /attachment/content/{id} > file` writes the response body
byte-for-byte, since non-JSON responses are passed through verbatim.

## License

MIT — see [LICENSE](LICENSE).
