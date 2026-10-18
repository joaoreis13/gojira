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

## checklist (Smart Checklist items)

`gojira checklist <subcommand> [flags]` — reads and writes the Railsware
Smart Checklist "Checklists" custom field. Item lines are markdown with a
status marker: `- ` todo, `+ ` done, `x ` skipped, `~ ` in progress.

- `gojira checklist get <issue> [--field <id>] [--output json|text] [--profile]`
  — `GET /issue/{key}?fields=<Checklists field>`; prints numbered items,
  sections and `done/total`. `--output json` emits
  `{issue, field:{id,name}, items:[{number,status,text,section}], done, total}`.
- `gojira checklist fields [--output json|text] [--profile]` — `GET /field`,
  listing every checklist custom field; the read/write one is marked.
- `gojira checklist set <issue> [--file <path>|--data '<markdown>'|--data -]
  [--field] [--dry-run] [--notify] [--profile]` — `PUT /issue/{key}` with the
  whole checklist. Content with no item lines is rejected.
- `gojira checklist add <issue> <text>... [--status <status>] [--field]
  [--dry-run] [--notify] [--profile]` — append an item (default `todo`).
- `gojira checklist check|uncheck|skip|start <issue> <item> [--field]
  [--dry-run] [--notify] [--profile]` — set done / todo / skipped /
  in-progress for one item.
- `gojira checklist remove <issue> <item> [--field] [--dry-run] [--notify]
  [--profile]` — delete one item (and its `>` detail lines).
- `gojira checklist raw <issue> [--profile]` — the raw
  `com.railsware.SmartChecklist.checklist` issue property, read-only.

`<item>` is a 1-based item number or matching item text (case-insensitive;
exact match wins over substring). Ambiguous or missing selectors are errors,
and no write happens. `remove` is stricter: number or exact text only.

Writes auto-target only a field named exactly "Checklists" (no fuzzy match)
and reject a `--field` that is not a `customfield_*` id unless
`--allow-any-field` is passed. Text output escapes control characters from
Jira data before printing. Mutations are read-modify-write:
they read the field, apply one change, and PUT the whole checklist back, so
it is worth a re-read (`checklist get`) before mutating a checklist someone
else may have just edited.
