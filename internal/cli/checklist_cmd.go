package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/joaoreis13/gojira/internal/checklist"
	"github.com/joaoreis13/gojira/internal/client"
	"github.com/joaoreis13/gojira/internal/output"
)

// checklistClient and checklistOut are seams over profileClient/os.Stdout so
// the checklist command tests can run against an httptest server and a buffer.
var (
	checklistClient           = profileClient
	checklistOut    io.Writer = os.Stdout
)

// checklistPropertyKey is the issue entity property Smart Checklist mirrors
// its state into (read-only for gojira: the documented write path is the
// "Checklists" custom field).
const checklistPropertyKey = "com.railsware.SmartChecklist.checklist"

type checklistOpts struct {
	Profile       string
	Field         string
	Output        string
	DryRun        bool
	Notify        bool
	AllowAnyField bool
	Status        checklist.Status
}

// customFieldID matches a Jira custom field id. Writes require it unless
// --allow-any-field is passed, so a mistyped --field (e.g. "summary") can
// never overwrite a non-checklist system field.
var customFieldID = regexp.MustCompile(`^customfield_\d+$`)

var (
	checklistProfileFlag  string
	checklistFieldFlag    string
	checklistOutputFlag   string
	checklistDryRunFlag   bool
	checklistNotifyFlag   bool
	checklistStatusFlag   string
	checklistFileFlag     string
	checklistDataFlag     string
	checklistAnyFieldFlag bool
)

var checklistCmd = &cobra.Command{
	Use:   "checklist",
	Short: "Read and edit Smart Checklist items on an issue",
	Long: `Read and edit Railsware Smart Checklist items through the Jira REST API.

The checklist lives in the app's "Checklists" custom field as one item per
line, with a status marker as the first character:

  "- " todo        "+ " done        "x " skipped        "~ " in progress

Headings ("#", "##"), item detail lines ("> ...") and blank lines are
preserved verbatim, so editing one item never reformats the rest of the
checklist. Selectors can be an item number or (a uniquely matching piece of)
item text — an ambiguous selector is an error, never a guess.

The field id is auto-detected from GET /field; pass --field when a site has
more than one checklist field. Reads default to the custom field, because
that is the app's documented read AND write path; "checklist raw" exposes the
com.railsware.SmartChecklist.checklist issue property instead.

Jira REST API notes for this app:
https://railsware.atlassian.net/wiki/spaces/CHK/pages/92176610`,
}

var checklistGetCmd = &cobra.Command{
	Use:   "get <issue>",
	Short: "Show an issue's checklist (GET /issue/{key})",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runChecklistGet(cmd.Context(), currentChecklistOpts(), args[0])
	},
}

var checklistFieldsCmd = &cobra.Command{
	Use:   "fields",
	Short: "List checklist custom fields on this Jira site (GET /field)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		return runChecklistFields(cmd.Context(), currentChecklistOpts())
	},
}

var checklistSetCmd = &cobra.Command{
	Use:   "set <issue>",
	Short: "Replace an issue's checklist from markdown (PUT /issue/{key})",
	Long: `Replace the whole checklist with markdown supplied via --file, --data, or
--data - (stdin). Each line starting with "- ", "+ ", "x " or "~ " becomes an
item; other lines are kept as-is.

Content with no item lines is rejected: replacing a checklist with an empty
value is almost always an accident.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		content, err := checklistSetContent(checklistFileFlag, checklistDataFlag)
		if err != nil {
			return err
		}
		return runChecklistSet(cmd.Context(), currentChecklistOpts(), args[0], content)
	},
}

var checklistAddCmd = &cobra.Command{
	Use:   "add <issue> <text>...",
	Short: "Append an item to an issue's checklist",
	Args:  cobra.MinimumNArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		opts := currentChecklistOpts()
		st, err := checklist.ParseStatus(checklistStatusFlag)
		if err != nil {
			return err
		}
		opts.Status = st
		return runChecklistAdd(cmd.Context(), opts, args[0], strings.Join(args[1:], " "))
	},
}

var checklistCheckCmd = statusCmd("check", "Mark an item done", checklist.Done)
var checklistUncheckCmd = statusCmd("uncheck", "Mark an item todo (unchecked)", checklist.Todo)
var checklistSkipCmd = statusCmd("skip", "Mark an item skipped", checklist.Skipped)
var checklistStartCmd = statusCmd("start", "Mark an item in progress", checklist.InProgress)

var checklistRemoveCmd = &cobra.Command{
	Use:   "remove <issue> <item>",
	Short: "Delete an item from an issue's checklist",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runChecklistRemove(cmd.Context(), currentChecklistOpts(), args[0], args[1])
	},
}

var checklistRawCmd = &cobra.Command{
	Use:   "raw <issue>",
	Short: "Print the raw com.railsware.SmartChecklist.checklist issue property",
	Long: `GET /issue/{issueIdOrKey}/properties/com.railsware.SmartChecklist.checklist.

This is the app's internal, JQL-optimized representation. gojira does not
write it (the app documents writes through the "Checklists" custom field
only); it is exposed for inspection and troubleshooting.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runChecklistRaw(cmd.Context(), currentChecklistOpts(), args[0])
	},
}

func statusCmd(use, short string, status checklist.Status) *cobra.Command {
	return &cobra.Command{
		Use:   use + " <issue> <item>",
		Short: short,
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := currentChecklistOpts()
			return runChecklistStatus(cmd.Context(), opts, args[0], args[1], status)
		},
	}
}

func init() {
	checklistCmd.AddCommand(
		checklistGetCmd, checklistFieldsCmd, checklistSetCmd, checklistAddCmd,
		checklistCheckCmd, checklistUncheckCmd, checklistSkipCmd, checklistStartCmd,
		checklistRemoveCmd, checklistRawCmd,
	)

	for _, c := range []*cobra.Command{checklistGetCmd, checklistFieldsCmd, checklistRawCmd} {
		c.Flags().StringVar(&checklistProfileFlag, "profile", "", "profile alias (defaults to the active profile)")
	}
	checklistGetCmd.Flags().StringVar(&checklistFieldFlag, "field", "", "checklist custom field id (defaults to auto-detected \"Checklists\")")
	checklistGetCmd.Flags().StringVar(&checklistOutputFlag, "output", "text", "output format: json or text")
	checklistFieldsCmd.Flags().StringVar(&checklistOutputFlag, "output", "text", "output format: json or text")

	for _, c := range []*cobra.Command{
		checklistSetCmd, checklistAddCmd, checklistCheckCmd, checklistUncheckCmd,
		checklistSkipCmd, checklistStartCmd, checklistRemoveCmd,
	} {
		c.Flags().StringVar(&checklistProfileFlag, "profile", "", "profile alias (defaults to the active profile)")
		c.Flags().StringVar(&checklistFieldFlag, "field", "", "checklist custom field id (defaults to auto-detected \"Checklists\")")
		c.Flags().BoolVar(&checklistDryRunFlag, "dry-run", false, "print the resulting checklist instead of writing it")
		c.Flags().BoolVar(&checklistNotifyFlag, "notify", false, "ask Jira to notify watchers about the update (best effort; Jira may ignore it without permission)")
		c.Flags().BoolVar(&checklistAnyFieldFlag, "allow-any-field", false, "let --field target a non-custom-field id (dangerous escape hatch)")
	}

	checklistSetCmd.Flags().StringVar(&checklistFileFlag, "file", "", "read checklist markdown from a file")
	checklistSetCmd.Flags().StringVar(&checklistDataFlag, "data", "", "checklist markdown inline, or - for stdin")
	checklistAddCmd.Flags().StringVar(&checklistStatusFlag, "status", "todo", "status for the new item: todo, done, skipped or in-progress")
}

func currentChecklistOpts() checklistOpts {
	st, err := checklist.ParseStatus(checklistStatusFlag)
	if err != nil {
		// Unparseable --status is caught when the command runs; fall back to
		// todo here so get/fields/raw are unaffected by the flag's default.
		st = checklist.Todo
	}
	return checklistOpts{
		Profile:       checklistProfileFlag,
		Field:         checklistFieldFlag,
		Output:        checklistOutputFlag,
		DryRun:        checklistDryRunFlag,
		Notify:        checklistNotifyFlag,
		AllowAnyField: checklistAnyFieldFlag,
		Status:        st,
	}
}

// --- field discovery --------------------------------------------------------

type jiraField struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Custom bool   `json:"custom"`
}

func isChecklistsField(f jiraField) bool {
	n := strings.ToLower(strings.TrimSpace(f.Name))
	if n == "checklists" {
		return true
	}
	return strings.Contains(n, "checklist") &&
		!strings.Contains(n, "progress") &&
		n != "smart checklist"
}

func listFields(ctx context.Context, c *client.Client) ([]jiraField, error) {
	resp, err := c.Do(ctx, http.MethodGet, "/field", nil, nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("GET /field -> HTTP %d: %s", resp.StatusCode, sanitizeText(strings.TrimSpace(string(resp.Body))))
	}
	var fields []jiraField
	if err := json.Unmarshal(resp.Body, &fields); err != nil {
		return nil, fmt.Errorf("decode field list: %w", err)
	}
	return fields, nil
}

// resolveChecklistField returns the custom field id/name to read or write.
// An explicit --field wins; otherwise the field named "Checklists" is used,
// and a site with more than one candidate is an error rather than a guess.
func resolveChecklistField(ctx context.Context, c *client.Client, opts checklistOpts, forWrite bool) (id, name string, err error) {
	explicit := opts.Field
	if explicit != "" {
		if forWrite && !customFieldID.MatchString(explicit) && !opts.AllowAnyField {
			return "", "", fmt.Errorf("refusing to write to %q: --field must be a custom field id (customfield_XXXXX); use --allow-any-field only if you really mean this field", explicit)
		}
		return explicit, explicit, nil
	}
	fields, err := listFields(ctx, c)
	if err != nil {
		return "", "", err
	}

	var exact, loose []jiraField
	for _, f := range fields {
		if !isChecklistsField(f) {
			continue
		}
		if strings.EqualFold(strings.TrimSpace(f.Name), "Checklists") {
			exact = append(exact, f)
			continue
		}
		loose = append(loose, f)
	}
	candidates := exact
	if len(candidates) == 0 && !forWrite {
		candidates = loose
	}

	switch len(candidates) {
	case 1:
		return candidates[0].ID, candidates[0].Name, nil
	case 0:
		if forWrite {
			return "", "", fmt.Errorf("no custom field named \"Checklists\" found; refusing to guess a write target (renamed field? pass --field <customfield_id>)")
		}
		return "", "", fmt.Errorf("no Smart Checklist \"Checklists\" custom field found on this Jira site (is Smart Checklist for Jira installed?); pass --field <customfield_id> to target one explicitly")
	default:
		parts := make([]string, 0, len(candidates))
		for _, f := range candidates {
			parts = append(parts, fmt.Sprintf("%s (%q)", f.ID, f.Name))
		}
		return "", "", fmt.Errorf("multiple checklist custom fields found: %s; pass --field <customfield_id>", strings.Join(parts, ", "))
	}
}

// --- REST helpers -----------------------------------------------------------

func readChecklistField(ctx context.Context, c *client.Client, issue, fieldID string) (string, error) {
	path := "/issue/" + url.PathEscape(issue)
	query := url.Values{"fields": []string{fieldID}}
	resp, err := c.Do(ctx, http.MethodGet, path, query, nil)
	if err != nil {
		return "", err
	}
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("GET %s?fields=%s -> HTTP %d: %s", path, fieldID, resp.StatusCode, sanitizeText(strings.TrimSpace(string(resp.Body))))
	}
	var payload struct {
		Fields map[string]json.RawMessage `json:"fields"`
	}
	if err := json.Unmarshal(resp.Body, &payload); err != nil {
		return "", fmt.Errorf("decode issue %s: %w", issue, err)
	}
	raw, ok := payload.Fields[fieldID]
	if !ok {
		return "", fmt.Errorf("issue %s response did not include field %s (is it on the issue screen and in the field's project context?)", issue, fieldID)
	}
	text, err := checklist.ExtractText(raw)
	if err != nil {
		return "", fmt.Errorf("issue %s field %s: %w", issue, fieldID, err)
	}
	return text, nil
}

func writeChecklistField(ctx context.Context, c *client.Client, issue, fieldID, text string, notify bool) error {
	body, err := json.Marshal(map[string]any{"fields": map[string]any{fieldID: text}})
	if err != nil {
		return err
	}
	query := url.Values{}
	if !notify {
		query.Set("notifyUsers", "false")
	}
	path := "/issue/" + url.PathEscape(issue)
	resp, err := c.Do(ctx, http.MethodPut, path, query, body)
	if err != nil {
		return err
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("PUT %s -> HTTP %d: %s", path, resp.StatusCode, sanitizeText(strings.TrimSpace(string(resp.Body))))
	}
	return nil
}

// --- command implementations ------------------------------------------------

type checklistItemJSON struct {
	Number       int    `json:"number"`
	Status       string `json:"status"`
	Text         string `json:"text"`
	Section      string `json:"section,omitempty"`
	SectionLevel int    `json:"sectionLevel,omitempty"`
}

type checklistReportJSON struct {
	Issue string `json:"issue"`
	Field struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"field"`
	Items []checklistItemJSON `json:"items"`
	Done  int                 `json:"done"`
	Total int                 `json:"total"`
}

func runChecklistGet(ctx context.Context, opts checklistOpts, issue string) error {
	c, alias, err := checklistClient(ctx, opts.Profile)
	if err != nil {
		return err
	}
	fieldID, fieldName, err := resolveChecklistField(ctx, c, opts, false)
	if err != nil {
		return err
	}
	text, err := readChecklistField(ctx, c, issue, fieldID)
	if err != nil {
		return fmt.Errorf("profile %q: %w", alias, err)
	}
	doc := checklist.Parse(text)
	done, total := doc.Progress()

	if opts.Output == "json" {
		payload := checklistReportJSON{Issue: issue, Done: done, Total: total}
		payload.Field.ID, payload.Field.Name = fieldID, fieldName
		payload.Items = make([]checklistItemJSON, 0, total)
		for _, it := range doc.Items() {
			payload.Items = append(payload.Items, checklistItemJSON{
				Number: it.Number, Status: string(it.Status), Text: it.Text,
				Section: it.Section, SectionLevel: it.SectionLevel,
			})
		}
		return output.Encode(checklistOut, payload, false)
	}

	fmt.Fprintf(checklistOut, "%s checklist — %s (%s): %d/%d done\n",
		sanitizeText(issue), sanitizeText(fieldName), sanitizeText(fieldID), done, total)
	printChecklistItems(doc)
	return nil
}

func printChecklistItems(doc *checklist.Doc) {
	items := doc.Items()
	if len(items) == 0 {
		fmt.Fprintln(checklistOut, "  (no items)")
		return
	}
	section := ""
	for _, it := range items {
		if it.Section != section {
			if it.Section != "" {
				fmt.Fprintf(checklistOut, "  %s %s\n", strings.Repeat("#", it.SectionLevel), sanitizeText(it.Section))
			}
			section = it.Section
		}
		fmt.Fprintf(checklistOut, "  %d. [%s] %s\n", it.Number, statusGlyph(it.Status), sanitizeText(it.Text))
	}
}

// sanitizeText escapes control characters (ANSI/OSC escapes, C0/C1, line
// separators) coming from Jira data before it reaches a terminal. Checklist
// text is editable by anyone who can edit the issue, so printing it raw lets
// them retitle the terminal, erase lines, or forge status output.
func sanitizeText(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		switch {
		case r == '\t':
			b.WriteString("\\t")
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f) || r == 0x2028 || r == 0x2029:
			fmt.Fprintf(&b, "\\u%04x", r)
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

func statusGlyph(s checklist.Status) string {
	switch s {
	case checklist.Done:
		return "x"
	case checklist.Skipped:
		return "-"
	case checklist.InProgress:
		return "~"
	default:
		return " "
	}
}

func runChecklistFields(ctx context.Context, opts checklistOpts) error {
	c, alias, err := checklistClient(ctx, opts.Profile)
	if err != nil {
		return err
	}
	fields, err := listFields(ctx, c)
	if err != nil {
		return fmt.Errorf("profile %q: %w", alias, err)
	}
	var found []jiraField
	for _, f := range fields {
		if strings.Contains(strings.ToLower(f.Name), "checklist") {
			found = append(found, f)
		}
	}
	if len(found) == 0 {
		fmt.Fprintln(checklistOut, "no checklist custom fields found; is Smart Checklist for Jira installed on this site?")
		return nil
	}
	sort.Slice(found, func(i, j int) bool { return found[i].Name < found[j].Name })

	if opts.Output == "json" {
		rows := make([]map[string]any, 0, len(found))
		for _, f := range found {
			rows = append(rows, map[string]any{"id": f.ID, "name": f.Name, "writable": isChecklistsField(f)})
		}
		return output.Encode(checklistOut, rows, false)
	}
	for _, f := range found {
		marker := ""
		if isChecklistsField(f) {
			marker = " (read/write)"
		}
		fmt.Fprintf(checklistOut, "%s\t%s%s\n", sanitizeText(f.ID), sanitizeText(f.Name), marker)
	}
	return nil
}

func checklistSetContent(file, data string) (string, error) {
	switch {
	case file != "" && data != "":
		return "", fmt.Errorf("pass only one of --file and --data")
	case file != "":
		b, err := os.ReadFile(file)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", file, err)
		}
		return string(b), nil
	case data == "-":
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		return string(b), nil
	case data != "":
		return data, nil
	default:
		return "", fmt.Errorf("no checklist content: pass --file <path>, --data '<markdown>', or --data - for stdin")
	}
}

// validateWriteFieldFlag fails fast on an obviously wrong --field, before any
// credential or network work happens.
func validateWriteFieldFlag(opts checklistOpts) error {
	if opts.Field != "" && !customFieldID.MatchString(opts.Field) && !opts.AllowAnyField {
		return fmt.Errorf("refusing to write to %q: --field must be a custom field id (customfield_XXXXX); use --allow-any-field only if you really mean this field", opts.Field)
	}
	return nil
}

func runChecklistSet(ctx context.Context, opts checklistOpts, issue, content string) error {
	if err := validateWriteFieldFlag(opts); err != nil {
		return err
	}
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("refusing to write an empty checklist; pass --file or --data with content")
	}
	doc := checklist.Parse(content)
	if doc.Len() == 0 {
		return fmt.Errorf("refusing to write checklist content with no items: prefix each item with \"- \", \"+ \", \"x \" or \"~ \"")
	}

	c, alias, err := checklistClient(ctx, opts.Profile)
	if err != nil {
		return err
	}
	fieldID, fieldName, err := resolveChecklistField(ctx, c, opts, true)
	if err != nil {
		return err
	}

	if opts.DryRun {
		fmt.Fprintf(checklistOut, "dry run — would write %d item(s) to %s (%s) on %s:\n%s",
			doc.Len(), sanitizeText(fieldName), sanitizeText(fieldID), sanitizeText(issue), doc.String())
		return nil
	}
	if err := writeChecklistField(ctx, c, issue, fieldID, doc.String(), opts.Notify); err != nil {
		return fmt.Errorf("profile %q: %w", alias, err)
	}
	done, total := doc.Progress()
	fmt.Fprintf(checklistOut, "updated %s checklist (%s, %s on profile %q): %d item(s), %d/%d done\n",
		sanitizeText(issue), sanitizeText(fieldName), sanitizeText(fieldID), alias, doc.Len(), done, total)
	return nil
}

func runChecklistAdd(ctx context.Context, opts checklistOpts, issue, text string) error {
	if opts.Status == "" {
		opts.Status = checklist.Todo
	}
	if !opts.Status.Valid() {
		return fmt.Errorf("invalid --status %q (want todo, done, skipped or in-progress)", opts.Status)
	}
	return mutateChecklist(ctx, opts, issue, func(doc *checklist.Doc) (int, string, error) {
		n, err := doc.Add(text, opts.Status)
		if err != nil {
			return 0, "", err
		}
		return n, fmt.Sprintf("added item %d to %s checklist", n, issue), nil
	})
}

func runChecklistStatus(ctx context.Context, opts checklistOpts, issue, selector string, status checklist.Status) error {
	return mutateChecklist(ctx, opts, issue, func(doc *checklist.Doc) (int, string, error) {
		n, err := doc.Select(selector)
		if err != nil {
			return 0, "", err
		}
		if err := doc.SetStatus(n, status); err != nil {
			return 0, "", err
		}
		done, total := doc.Progress()
		return n, fmt.Sprintf("%s item %d -> %s (%d/%d done)", issue, n, status, done, total), nil
	})
}

// mutateChecklist implements the shared read-modify-write cycle: resolve the
// field, read the current checklist, let fn mutate it, then write it back.
// fn runs before any write, so a bad selector can never half-apply a change.
func mutateChecklist(ctx context.Context, opts checklistOpts, issue string, fn func(*checklist.Doc) (int, string, error)) error {
	if err := validateWriteFieldFlag(opts); err != nil {
		return err
	}
	c, alias, err := checklistClient(ctx, opts.Profile)
	if err != nil {
		return err
	}
	fieldID, fieldName, err := resolveChecklistField(ctx, c, opts, false)
	if err != nil {
		return err
	}
	text, err := readChecklistField(ctx, c, issue, fieldID)
	if err != nil {
		return fmt.Errorf("profile %q: %w", alias, err)
	}
	doc := checklist.Parse(text)
	if _, msg, err := fn(doc); err != nil {
		return err
	} else if opts.DryRun {
		fmt.Fprintf(checklistOut, "dry run — %s\n%s", msg, doc.String())
		return nil
	}

	if err := writeChecklistField(ctx, c, issue, fieldID, doc.String(), opts.Notify); err != nil {
		return fmt.Errorf("profile %q: %w", alias, err)
	}
	done, total := doc.Progress()
	fmt.Fprintf(checklistOut, "%s checklist (%s) on profile %q: %d item(s), %d/%d done\n",
		sanitizeText(issue), sanitizeText(fieldName), alias, doc.Len(), done, total)
	printChecklistItems(doc)
	return nil
}

func runChecklistRemove(ctx context.Context, opts checklistOpts, issue, selector string) error {
	return mutateChecklist(ctx, opts, issue, func(doc *checklist.Doc) (int, string, error) {
		n, err := doc.SelectExact(selector)
		if err != nil {
			return 0, "", err
		}
		items := doc.Items()
		text := ""
		if n-1 < len(items) {
			text = items[n-1].Text
		}
		if err := doc.Remove(n); err != nil {
			return 0, "", err
		}
		return n, fmt.Sprintf("removed item %d (%q) from %s checklist", n, text, issue), nil
	})
}

func runChecklistRaw(ctx context.Context, opts checklistOpts, issue string) error {
	c, alias, err := checklistClient(ctx, opts.Profile)
	if err != nil {
		return err
	}
	path := "/issue/" + url.PathEscape(issue) + "/properties/" + checklistPropertyKey
	resp, err := c.Do(ctx, http.MethodGet, path, nil, nil)
	if err != nil {
		return err
	}
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("issue %s has no %s property (the checklist may never have been opened in the app)", issue, checklistPropertyKey)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("GET %s -> HTTP %d on profile %q: %s", path, resp.StatusCode, alias, sanitizeText(strings.TrimSpace(string(resp.Body))))
	}
	return renderResponse(checklistOut, resp.Body, "", "json", true)
}
