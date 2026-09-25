# gojira command reference

Static copy of `gojira <command> --help` for every command, kept short.
When in doubt, run `--help` on the live binary — it's authoritative.

## profile

A profile is a base URL + OAuth app + authenticated user. Exactly one
profile is active at a time; switching is always a manual, explicit step
(`gojira profile use`) — nothing switches it automatically.

- `gojira profile add <alias> --base-url <url> --client-id <id> --client-secret <secret> [--scopes a,b,c] [--redirect-port N] [--activate]`
  Register a Jira profile's OAuth app config. Both `--client-id` and
  `--client-secret` are required (Atlassian 3LO apps are confidential
  clients only). Never activates the profile unless `--activate` is passed
  — this holds even for the very first profile added.
- `gojira profile list` — list configured profiles, marking the active one
  and its cached cloud id.
- `gojira profile use [alias]` — set the active profile. With no alias,
  lists profiles and prompts for one (needs an interactive terminal; pass
  the alias directly otherwise). Activating a profile immediately forces a
  token refresh for it; profiles you don't switch to are never touched. If
  the refresh fails (refresh token expired from long disuse), the switch
  still takes effect but you'll need `gojira auth login` again.
- `gojira profile remove <alias>` — remove a profile's config and stored
  credentials.

## auth

- `gojira auth login [--profile <alias>] [--no-browser]` — browser OAuth
  login; stores tokens. `--no-browser` prints the authorization URL instead
  of opening it automatically (useful with multiple browsers/profiles).
- `gojira auth status [--profile <alias>]` — show token expiry and whether
  a refresh token is present.
- `gojira auth refresh [--profile <alias>]` — force an immediate refresh.
- `gojira auth logout [--profile <alias>]` — delete local credentials only
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
- `--profile <alias>` — which configured profile to use for this call only
  (defaults to the active profile; doesn't change what's active).

## whoami

`gojira whoami [--profile <alias>]` — `GET /myself`, printed with
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
- `--profile <alias>`.
