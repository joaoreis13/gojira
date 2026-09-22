# gojira command reference

Static copy of `gojira <command> --help` for every command, kept short.
When in doubt, run `--help` on the live binary — it's authoritative.

## site

- `gojira site add <alias> --base-url <url> --client-id <id> --client-secret <secret> [--scopes a,b,c] [--redirect-port N] [--default]`
  Register a Jira site's OAuth app config. Both `--client-id` and
  `--client-secret` are required (Atlassian 3LO apps are confidential
  clients only).
- `gojira site list` — list configured sites, marking the default and
  cached cloud id.
- `gojira site use <alias>` — set the default site.
- `gojira site remove <alias>` — remove a site's config and stored
  credentials.

## auth

- `gojira auth login [--site <alias>]` — browser OAuth login; stores tokens.
- `gojira auth status [--site <alias>]` — show token expiry and whether a
  refresh token is present.
- `gojira auth refresh [--site <alias>]` — force an immediate refresh.
- `gojira auth logout [--site <alias>]` — delete local credentials only
  (does not revoke the grant on Atlassian's side).

## api (the generic passthrough — full endpoint coverage)

`gojira api <METHOD> <path> [flags]`

- `--data <value>` — request body: inline JSON, `@file`, or `-` for stdin.
- `--query key=value` — query parameter (repeatable).
- `--fields a,b.c` — comma-separated dot-paths to keep in the output.
  Applied per-item inside `issues`/`values` list envelopes.
- `--output json|text` — `text` renders one line per item for recognized
  shapes; default is `json`.
- `--pretty` — indent JSON output.
- `--yes` / `-y` — skip the confirmation prompt for destructive methods
  (currently: any `DELETE`).
- `--site <alias>` — which configured site to use (defaults to the
  default site).

## whoami

`gojira whoami [--site <alias>]` — `GET /myself`, printed with
`displayName,emailAddress,accountId` kept by default.

## search

`gojira search '<JQL>' [flags]` — `POST /search/jql`.

- `--max-results N` — page size (default 50).
- `--page-token <token>` — fetch the page after the given `nextPageToken`.
- `--jira-fields a,b,c` — Jira issue fields to request from the API
  (passed as the request's `fields`).
- `--fields a,b.c` — output field selection (same semantics as `api
  --fields`).
- `--output json|text` — default `text`.
- `--site <alias>`.
