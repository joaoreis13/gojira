// Package checklist parses and rewrites the Railsware Smart Checklist
// markdown format — the value of the "Checklists" custom field that gojira
// reads and writes through the Jira REST API.
//
// One checklist item per line, with a status marker as the first character:
//
//   - todo
//   - done / checked
//     x  skipped
//     ~  in progress
//
// Headings ("#", "##", ...), item detail lines ("> ..."), and blank lines are
// preserved verbatim so a read-modify-write round trip never reformats a
// checklist the user already has. See
// https://railsware.atlassian.net/wiki/spaces/CHK/pages/88866897
package checklist

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// Status is the state of a single checklist item.
type Status string

const (
	Todo       Status = "todo"
	Done       Status = "done"
	Skipped    Status = "skipped"
	InProgress Status = "in-progress"
)

var statusPrefixes = map[Status]string{
	Todo:       "- ",
	Done:       "+ ",
	Skipped:    "x ",
	InProgress: "~ ",
}

// statusMarkers is the bare marker byte written back when only an item's
// status changes, so the rest of the original line is preserved.
var statusMarkers = map[Status]byte{
	Todo:       '-',
	Done:       '+',
	Skipped:    'x',
	InProgress: '~',
}

var statusAliases = map[string]Status{
	"todo": Todo, "-": Todo, "": Todo, "open": Todo, "unchecked": Todo,
	"done": Done, "+": Done, "checked": Done, "check": Done, "complete": Done, "completed": Done,
	"skipped": Skipped, "skip": Skipped, "x": Skipped,
	"in-progress": InProgress, "in_progress": InProgress, "inprogress": InProgress,
	"progress": InProgress, "started": InProgress, "~": InProgress,
}

// ParseStatus maps a human/CLI spelling of a status to its canonical form.
func ParseStatus(s string) (Status, error) {
	if st, ok := statusAliases[strings.ToLower(strings.TrimSpace(s))]; ok {
		return st, nil
	}
	return "", fmt.Errorf("unknown checklist status %q (want todo, done, skipped or in-progress)", s)
}

// Prefix is the markdown prefix written for a status.
func (s Status) Prefix() string {
	if p, ok := statusPrefixes[s]; ok {
		return p
	}
	return statusPrefixes[Todo]
}

// Valid reports whether s is one of the four canonical statuses.
func (s Status) Valid() bool {
	_, ok := statusPrefixes[s]
	return ok
}

// markerStatus maps a leading marker byte to its status.
func markerStatus(b byte) (Status, bool) {
	switch b {
	case '-':
		return Todo, true
	case '+':
		return Done, true
	case 'x', 'X':
		return Skipped, true
	case '~':
		return InProgress, true
	}
	return "", false
}

// Kind classifies an input line.
type Kind int

const (
	KindBlank Kind = iota
	KindHeading
	KindItem
	KindDetail
	KindOther
)

// Line is one physical line of checklist markdown.
type Line struct {
	Kind         Kind
	Raw          string
	Status       Status // Item only
	Text         string // Item: item text without marker; Heading: heading text
	Section      string // Item: nearest preceding heading, "" if none
	SectionLevel int    // Item: level of that heading (1 for "#")
	Number       int    // Item: 1-based item number, 0 otherwise
	markerIdx    int    // Item: byte offset of the status marker within Raw
	dirty        bool   // Item: status was changed, so Raw no longer applies
}

// Item is the read-only view of a checklist item.
type Item struct {
	Number       int
	Status       Status
	Text         string
	Section      string
	SectionLevel int
}

// Doc is a parsed checklist that round-trips byte for byte unless mutated.
type Doc struct {
	lines    []Line
	trailing bool // input ended with a newline
}

// Parse reads checklist markdown. CRLF input is normalized to LF.
func Parse(text string) *Doc {
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	d := &Doc{}
	if text == "" {
		return d
	}
	raw := strings.Split(text, "\n")
	if raw[len(raw)-1] == "" {
		d.trailing = true
		raw = raw[:len(raw)-1]
	}

	section, level := "", 0
	number := 0
	for _, line := range raw {
		l := Line{Raw: line}
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			l.Kind = KindBlank
		case strings.HasPrefix(trimmed, "#"):
			l.Kind = KindHeading
			n := 0
			for n < len(trimmed) && trimmed[n] == '#' {
				n++
			}
			l.Text = strings.TrimSpace(trimmed[n:])
			section, level = l.Text, n
		case isItemLine(trimmed):
			number++
			l.Kind = KindItem
			st, _ := markerStatus(trimmed[0])
			l.Status = st
			l.markerIdx = strings.Index(line, trimmed)
			l.Text = strings.TrimSpace(trimmed[1:])
			l.Number = number
			l.Section, l.SectionLevel = section, level
		case strings.HasPrefix(trimmed, ">"):
			l.Kind = KindDetail
		default:
			l.Kind = KindOther
		}
		d.lines = append(d.lines, l)
	}
	return d
}

func isItemLine(trimmed string) bool {
	if len(trimmed) < 2 {
		return false
	}
	if _, ok := markerStatus(trimmed[0]); !ok {
		return false
	}
	return trimmed[1] == ' ' || trimmed[1] == '\t'
}

// Len is the number of items in the checklist.
func (d *Doc) Len() int {
	n := 0
	for _, l := range d.lines {
		if l.Kind == KindItem {
			n++
		}
	}
	return n
}

// Items returns every item in order.
func (d *Doc) Items() []Item {
	items := make([]Item, 0, d.Len())
	for _, l := range d.lines {
		if l.Kind == KindItem {
			items = append(items, Item{
				Number:       l.Number,
				Status:       l.Status,
				Text:         l.Text,
				Section:      l.Section,
				SectionLevel: l.SectionLevel,
			})
		}
	}
	return items
}

// String serializes the document back to checklist markdown.
func (d *Doc) String() string {
	if len(d.lines) == 0 {
		return ""
	}
	var b strings.Builder
	for i, l := range d.lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		if l.Kind == KindItem && l.dirty && l.markerIdx < len(l.Raw) {
			// Swap only the marker byte: indentation, marker casing of other
			// items, separator whitespace and trailing spaces all survive, so a
			// one-item edit never reformats the rest of somebody's checklist.
			b.WriteString(l.Raw[:l.markerIdx])
			b.WriteByte(statusMarkers[l.Status])
			b.WriteString(l.Raw[l.markerIdx+1:])
		} else {
			b.WriteString(l.Raw)
		}
	}
	if d.trailing {
		b.WriteByte('\n')
	}
	return b.String()
}

// SetStatus rewrites the marker of item number (1-based).
func (d *Doc) SetStatus(number int, s Status) error {
	if !s.Valid() {
		return fmt.Errorf("invalid checklist status %q", s)
	}
	for i := range d.lines {
		if d.lines[i].Kind == KindItem && d.lines[i].Number == number {
			d.lines[i].Status = s
			d.lines[i].dirty = true
			return nil
		}
	}
	return fmt.Errorf("no checklist item %d (checklist has %d item(s))", number, d.Len())
}

// Add appends an item and returns its number.
func (d *Doc) Add(text string, s Status) (int, error) {
	if !s.Valid() {
		return 0, fmt.Errorf("invalid checklist status %q", s)
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return 0, fmt.Errorf("checklist item text is empty")
	}
	if err := validateItemText(text); err != nil {
		return 0, err
	}
	number := d.Len() + 1
	section, level := "", 0
	for _, l := range d.lines {
		if l.Kind == KindHeading {
			trimmed := strings.TrimSpace(l.Raw)
			level = 0
			for level < len(trimmed) && trimmed[level] == '#' {
				level++
			}
			section = l.Text
		}
	}
	d.lines = append(d.lines, Line{
		Kind:         KindItem,
		Raw:          s.Prefix() + text,
		Status:       s,
		Text:         text,
		Number:       number,
		Section:      section,
		SectionLevel: level,
		markerIdx:    0,
		dirty:        true,
	})
	d.trailing = true
	return number, nil
}

// Remove deletes item number (1-based) along with its detail lines.
func (d *Doc) Remove(number int) error {
	for i, l := range d.lines {
		if l.Kind != KindItem || l.Number != number {
			continue
		}
		end := i + 1
		for end < len(d.lines) && d.lines[end].Kind == KindDetail {
			end++
		}
		d.lines = append(d.lines[:i], d.lines[end:]...)
		d.renumber()
		return nil
	}
	return fmt.Errorf("no checklist item %d (checklist has %d item(s))", number, d.Len())
}

func (d *Doc) renumber() {
	n := 0
	for i := range d.lines {
		if d.lines[i].Kind == KindItem {
			n++
			d.lines[i].Number = n
		}
	}
}

// Progress counts done items against all items.
func (d *Doc) Progress() (done, total int) {
	for _, l := range d.lines {
		if l.Kind != KindItem {
			continue
		}
		total++
		if l.Status == Done {
			done++
		}
	}
	return done, total
}

// Select resolves a selector to an item number. A numeric selector is an item
// number; otherwise an exact case-insensitive text match wins, then a unique
// case-insensitive substring match. Ambiguous and missing selectors are
// errors, so a caller never mutates the wrong item by accident.
func (d *Doc) Select(selector string) (int, error) {
	sel := strings.TrimSpace(selector)
	if sel == "" {
		return 0, fmt.Errorf("empty checklist item selector")
	}
	if n, err := strconv.Atoi(sel); err == nil {
		if n < 1 || n > d.Len() {
			return 0, fmt.Errorf("checklist item %d does not exist (checklist has %d item(s))", n, d.Len())
		}
		return n, nil
	}

	items := d.Items()
	var exact []Item
	for _, it := range items {
		if strings.EqualFold(it.Text, sel) {
			exact = append(exact, it)
		}
	}
	if len(exact) == 1 {
		return exact[0].Number, nil
	}
	if len(exact) > 1 {
		return 0, ambiguousError(sel, exact)
	}

	var partial []Item
	lower := strings.ToLower(sel)
	for _, it := range items {
		if strings.Contains(strings.ToLower(it.Text), lower) {
			partial = append(partial, it)
		}
	}
	switch len(partial) {
	case 0:
		return 0, fmt.Errorf("no checklist item matches %q (checklist has %d item(s))", sel, d.Len())
	case 1:
		return partial[0].Number, nil
	default:
		return 0, ambiguousError(sel, partial)
	}
}

func ambiguousError(sel string, items []Item) error {
	parts := make([]string, 0, len(items))
	for _, it := range items {
		parts = append(parts, fmt.Sprintf("%d (%q)", it.Number, it.Text))
	}
	return fmt.Errorf("ambiguous checklist item selector %q: matches items %s; use the item number instead",
		sel, strings.Join(parts, ", "))
}

// SelectExact is Select without the substring fallback: a numeric item number
// or the item's full text (case-insensitive). Destructive callers use it so a
// vague selector can never delete the wrong item.
func (d *Doc) SelectExact(selector string) (int, error) {
	sel := strings.TrimSpace(selector)
	if sel == "" {
		return 0, fmt.Errorf("empty checklist item selector")
	}
	if n, err := strconv.Atoi(sel); err == nil {
		if n < 1 || n > d.Len() {
			return 0, fmt.Errorf("checklist item %d does not exist (checklist has %d item(s))", n, d.Len())
		}
		return n, nil
	}

	var exact []Item
	for _, it := range d.Items() {
		if strings.EqualFold(it.Text, sel) {
			exact = append(exact, it)
		}
	}
	switch len(exact) {
	case 1:
		return exact[0].Number, nil
	case 0:
		return 0, fmt.Errorf("no checklist item exactly matches %q; pass the full item text or its item number", sel)
	default:
		return 0, ambiguousError(sel, exact)
	}
}

// validateItemText rejects anything that would forge extra checklist lines or
// smuggle control sequences into Jira's field value once written.
func validateItemText(text string) error {
	for _, r := range text {
		switch {
		case r == '\n' || r == '\r':
			return fmt.Errorf("checklist item text must be a single line, got %q", text)
		case r == 0x2028 || r == 0x2029 || r == 0x85:
			return fmt.Errorf("checklist item text must not contain line separators (U+%04X), got %q", r, text)
		case r < 0x20 || r == 0x7f || (r >= 0x80 && r <= 0x9f):
			return fmt.Errorf("checklist item text must not contain control characters (U+%04X), got %q", r, text)
		}
	}
	return nil
}

// String renders a status for JSON output.
func (s Status) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(s))
}

// ExtractText pulls checklist markdown out of a custom field value, whose
// exact JSON shape depends on the Jira field type and app version: a plain
// string, an object wrapper ({"v": ...}, {"text": ...}, {"value": ...}), or
// Atlassian Document Format. Unknown shapes are an error rather than a silent
// empty checklist, so a schema change never looks like a wiped checklist.
func ExtractText(raw json.RawMessage) (string, error) {
	return extractText(raw, 0)
}

func extractText(raw json.RawMessage, depth int) (string, error) {
	if depth > 5 {
		return "", fmt.Errorf("checklist field value is nested too deeply")
	}
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", nil
	}

	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}

	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", fmt.Errorf("unsupported checklist field value: %s", trimmed)
	}
	for _, key := range []string{"v", "text", "value"} {
		if inner, ok := obj[key]; ok {
			if out, err := extractText(inner, depth+1); err == nil && out != "" {
				return out, nil
			}
		}
	}
	if _, ok := obj["type"]; ok {
		if out, ok := adfText(obj); ok {
			return out, nil
		}
	}
	return "", fmt.Errorf("unsupported checklist field value: %s", trimmed)
}

type adfNode struct {
	Type    string            `json:"type"`
	Text    string            `json:"text"`
	Content []json.RawMessage `json:"content"`
}

// adfText flattens an Atlassian Document Format node into checklist lines,
// one paragraph/heading per line.
func adfText(obj map[string]json.RawMessage) (string, bool) {
	var node adfNode
	raw, err := json.Marshal(obj)
	if err != nil {
		return "", false
	}
	if err := json.Unmarshal(raw, &node); err != nil {
		return "", false
	}
	if node.Type == "text" {
		return node.Text, true
	}
	if len(node.Content) == 0 {
		return node.Text, node.Text != ""
	}

	var parts []string
	for _, child := range node.Content {
		var c adfNode
		if err := json.Unmarshal(child, &c); err != nil {
			continue
		}
		if c.Type == "bulletList" || c.Type == "orderedList" {
			var items []string
			for _, li := range c.Content {
				var l adfNode
				if err := json.Unmarshal(li, &l); err != nil {
					continue
				}
				if t, ok := adfText(nodeMap(l)); ok && t != "" {
					items = append(items, "- "+t)
				}
			}
			if len(items) > 0 {
				parts = append(parts, strings.Join(items, "\n"))
			}
			continue
		}
		if t, ok := adfText(nodeMap(c)); ok && t != "" {
			parts = append(parts, t)
		}
	}
	if len(parts) == 0 {
		return "", node.Text != ""
	}
	return strings.Join(parts, "\n"), true
}

func nodeMap(n adfNode) map[string]json.RawMessage {
	raw, _ := json.Marshal(n)
	var m map[string]json.RawMessage
	_ = json.Unmarshal(raw, &m)
	return m
}
