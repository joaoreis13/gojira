package checklist

import (
	"encoding/json"
	"strings"
	"testing"
)

const sample = `# Release instructions
## Preparation
+ Add feature permissions
+ Send internal announcement
~ Prepare branch
> * merge feature to master
> * set tag

## Deploy
- Deploy master to prod
x Deploy new login page
- Test and confirm successful deployment
`

func TestParseMapsMarkersToStatuses(t *testing.T) {
	doc := Parse(sample)
	items := doc.Items()
	if len(items) != 6 {
		t.Fatalf("got %d items, want 6: %+v", len(items), items)
	}
	want := []struct {
		status Status
		text   string
	}{
		{Done, "Add feature permissions"},
		{Done, "Send internal announcement"},
		{InProgress, "Prepare branch"},
		{Todo, "Deploy master to prod"},
		{Skipped, "Deploy new login page"},
		{Todo, "Test and confirm successful deployment"},
	}
	for i, w := range want {
		if items[i].Status != w.status {
			t.Errorf("item %d status = %q, want %q", i+1, items[i].Status, w.status)
		}
		if items[i].Text != w.text {
			t.Errorf("item %d text = %q, want %q", i+1, items[i].Text, w.text)
		}
		if items[i].Number != i+1 {
			t.Errorf("item %d Number = %d, want %d", i+1, items[i].Number, i+1)
		}
	}
}

func TestParseAssignsSections(t *testing.T) {
	doc := Parse(sample)
	items := doc.Items()
	if items[0].Section != "Preparation" {
		t.Errorf("first section = %q, want %q", items[0].Section, "Preparation")
	}
	if items[3].Section != "Deploy" {
		t.Errorf("fourth section = %q, want %q", items[3].Section, "Deploy")
	}
	if items[3].SectionLevel != 2 {
		t.Errorf("Deploy heading level = %d, want 2", items[3].SectionLevel)
	}
}

func TestItemsWithoutHeadingHaveEmptySection(t *testing.T) {
	doc := Parse("- one\n- two\n")
	for _, it := range doc.Items() {
		if it.Section != "" || it.SectionLevel != 0 {
			t.Errorf("item %q section = %q/%d, want empty", it.Text, it.Section, it.SectionLevel)
		}
	}
}

func TestParseKeepsDetailLinesOutOfItems(t *testing.T) {
	doc := Parse(sample)
	for _, it := range doc.Items() {
		if strings.HasPrefix(it.Text, "*") || strings.Contains(it.Text, "merge feature") {
			t.Errorf("detail line leaked into items: %q", it.Text)
		}
	}
	if !strings.Contains(doc.String(), "> * merge feature to master") {
		t.Errorf("round-trip dropped a detail line:\n%s", doc.String())
	}
}

func TestRoundTripIsByteIdentical(t *testing.T) {
	if got := Parse(sample).String(); got != sample {
		t.Errorf("round-trip mismatch\n--- got ---\n%q\n--- want ---\n%q", got, sample)
	}
}

func TestParseToleratesCRLF(t *testing.T) {
	doc := Parse("- one\r\n+ two\r\n")
	if got := doc.Len(); got != 2 {
		t.Fatalf("Len = %d, want 2", got)
	}
	if items := doc.Items(); items[1].Text != "two" {
		t.Errorf("text = %q, want %q (no stray \\r)", items[1].Text, "two")
	}
	if strings.Contains(doc.String(), "\r") {
		t.Errorf("serialized output still contains CR: %q", doc.String())
	}
}

func TestParseStatusAliases(t *testing.T) {
	cases := map[string]Status{
		"todo":        Todo,
		"open":        Todo,
		"-":           Todo,
		"done":        Done,
		"checked":     Done,
		"check":       Done,
		"+":           Done,
		"skipped":     Skipped,
		"skip":        Skipped,
		"x":           Skipped,
		"in-progress": InProgress,
		"in_progress": InProgress,
		"progress":    InProgress,
		"~":           InProgress,
		"DONE":        Done,
	}
	for in, want := range cases {
		got, err := ParseStatus(in)
		if err != nil {
			t.Errorf("ParseStatus(%q) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ParseStatus(%q) = %q, want %q", in, got, want)
		}
	}
	if _, err := ParseStatus("bogus"); err == nil {
		t.Error("ParseStatus(\"bogus\") = nil error, want error")
	}
}

func TestSetStatusRewritesOnlyThatLine(t *testing.T) {
	doc := Parse(sample)
	if err := doc.SetStatus(4, Done); err != nil {
		t.Fatal(err)
	}
	got := doc.String()
	want := strings.Replace(sample, "- Deploy master to prod", "+ Deploy master to prod", 1)
	if got != want {
		t.Errorf("SetStatus output mismatch\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	if doc.Items()[3].Status != Done {
		t.Errorf("item 4 status = %q, want done", doc.Items()[3].Status)
	}
}

func TestSetStatusRejectsOutOfRange(t *testing.T) {
	doc := Parse(sample)
	if err := doc.SetStatus(0, Done); err == nil {
		t.Error("SetStatus(0) = nil error, want error")
	}
	if err := doc.SetStatus(99, Done); err == nil {
		t.Error("SetStatus(99) = nil error, want error")
	}
}

func TestAddAppendsItemAndPreservesExisting(t *testing.T) {
	doc := Parse(sample)
	n, err := doc.Add("Write release notes", InProgress)
	if err != nil {
		t.Fatal(err)
	}
	if n != 7 {
		t.Errorf("Add returned number %d, want 7", n)
	}
	out := doc.String()
	if !strings.HasSuffix(out, "~ Write release notes\n") {
		t.Errorf("output does not end with the new item: %q", out[len(out)-40:])
	}
	if !strings.HasPrefix(out, sample) {
		t.Error("Add rewrote existing content")
	}
}

func TestAddToEmptyChecklist(t *testing.T) {
	doc := Parse("")
	n, err := doc.Add("First item", Todo)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("number = %d, want 1", n)
	}
	if got := doc.String(); got != "- First item\n" {
		t.Errorf("output = %q", got)
	}
}

func TestAddRejectsNewlines(t *testing.T) {
	doc := Parse(sample)
	if _, err := doc.Add("line one\n- injected", Todo); err == nil {
		t.Error("Add accepted a newline, want error (item text must stay one line)")
	}
}

func TestRemoveDeletesItemAndKeepsText(t *testing.T) {
	doc := Parse(sample)
	if err := doc.Remove(2); err != nil {
		t.Fatal(err)
	}
	out := doc.String()
	if strings.Contains(out, "Send internal announcement") {
		t.Errorf("removed item still present:\n%s", out)
	}
	if got := doc.Len(); got != 5 {
		t.Errorf("Len = %d, want 5", got)
	}
	if err := doc.Remove(99); err == nil {
		t.Error("Remove(99) = nil error, want error")
	}
}

func TestProgressCountsDoneItems(t *testing.T) {
	done, total := Parse(sample).Progress()
	if done != 2 || total != 6 {
		t.Errorf("Progress = %d/%d, want 2/6", done, total)
	}
}

func TestSelectByNumberAndText(t *testing.T) {
	doc := Parse(sample)
	if n, err := doc.Select("3"); err != nil || n != 3 {
		t.Errorf("Select(\"3\") = %d, %v; want 3, nil", n, err)
	}
	if n, err := doc.Select("prepare BRANCH"); err != nil || n != 3 {
		t.Errorf("Select(case-insensitive exact) = %d, %v; want 3, nil", n, err)
	}
	if n, err := doc.Select("deploy master"); err != nil || n != 4 {
		t.Errorf("Select(substring) = %d, %v; want 4, nil", n, err)
	}
}

func TestSelectAmbiguousAndMissing(t *testing.T) {
	doc := Parse("- deploy api\n+ deploy web\n")
	if _, err := doc.Select("deploy"); err == nil {
		t.Error("Select with two matches = nil error, want ambiguity error")
	} else if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("ambiguity error = %q, want it to mention ambiguous", err)
	}
	if _, err := doc.Select("nope"); err == nil {
		t.Error("Select missing = nil error, want error")
	}
	if _, err := doc.Select("9"); err == nil {
		t.Error("Select out-of-range number = nil error, want error")
	}
}

func TestSelectExactBeatsSubstring(t *testing.T) {
	doc := Parse("- deploy\n- deploy api\n")
	n, err := doc.Select("deploy")
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("Select = %d, want 1 (exact match wins over substring)", n)
	}
}

func TestExtractTextShapes(t *testing.T) {
	cases := map[string]string{
		`"- one\n+ two\n"`:    "- one\n+ two\n",
		`{"v":"- one\n"}`:     "- one\n",
		`{"text":"- one\n"}`:  "- one\n",
		`{"value":"- one\n"}`: "- one\n",
		`null`:                "",
		`""`:                  "",
		`{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"- one"}]},{"type":"paragraph","content":[{"type":"text","text":"+ two"}]}]}`: "- one\n+ two",
	}
	for in, want := range cases {
		got, err := ExtractText(json.RawMessage(in))
		if err != nil {
			t.Errorf("ExtractText(%s) error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("ExtractText(%s) = %q, want %q", in, got, want)
		}
	}
}

func TestExtractTextRejectsUnknownShape(t *testing.T) {
	if _, err := ExtractText(json.RawMessage(`42`)); err == nil {
		t.Error("ExtractText(42) = nil error, want error")
	}
}

func TestHeadingOnlyAndEmptyDocs(t *testing.T) {
	if got := Parse("# Only a heading\n").String(); got != "# Only a heading\n" {
		t.Errorf("heading-only round-trip = %q", got)
	}
	if got := Parse("").String(); got != "" {
		t.Errorf("empty doc = %q, want empty", got)
	}
	if _, total := Parse("").Progress(); total != 0 {
		t.Errorf("empty doc total = %d, want 0", total)
	}
}

func TestStatusMarker(t *testing.T) {
	cases := map[Status]string{Todo: "- ", Done: "+ ", Skipped: "x ", InProgress: "~ "}
	for st, want := range cases {
		if got := st.Prefix(); got != want {
			t.Errorf("%q.Prefix() = %q, want %q", st, got, want)
		}
	}
}

// --- fidelity of a read-modify-write round trip -----------------------------

const shaggy = "   - indented item\nX upper marker\n-	tabbed  \n> detail  \n# C# notes\n- plain\n"

func TestRoundTripPreservesLineShape(t *testing.T) {
	if got := Parse(shaggy).String(); got != shaggy {
		t.Errorf("unmodified round trip changed bytes\n--- got ---\n%q\n--- want ---\n%q", got, shaggy)
	}
}

func TestSetStatusRewritesOnlyTheMarker(t *testing.T) {
	doc := Parse(shaggy)
	if err := doc.SetStatus(2, Done); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(doc.String(), "+ upper marker") {
		t.Errorf("mutated line lost its shape: %q", doc.String())
	}
	if !strings.Contains(doc.String(), "   - indented item") || !strings.Contains(doc.String(), "-	tabbed  ") {
		t.Errorf("untouched items were rewritten: %q", doc.String())
	}
}

func TestSetStatusKeepsInnerSpacing(t *testing.T) {
	doc := Parse("-   spaced text  \n")
	if err := doc.SetStatus(1, Done); err != nil {
		t.Fatal(err)
	}
	if got := doc.String(); got != "+   spaced text  \n" {
		t.Errorf("SetStatus = %q, want the marker swapped and spacing kept", got)
	}
}

func TestAddHeadingLevelCountsLeadingHashesOnly(t *testing.T) {
	doc := Parse("# C# notes\n")
	if _, err := doc.Add("new", Todo); err != nil {
		t.Fatal(err)
	}
	items := doc.Items()
	if items[0].SectionLevel != 1 {
		t.Errorf("SectionLevel = %d, want 1 (only leading # run counts)", items[0].SectionLevel)
	}
}

func TestAddRejectsControlAndLineSeparatorChars(t *testing.T) {
	for _, bad := range []string{"a\u2028- b", "a\u0085b", "a\x00b", "a\x1b[31mb", "a\rb", "a\nb"} {
		doc := Parse("- one\n")
		if _, err := doc.Add(bad, Todo); err == nil {
			t.Errorf("Add(%q) = nil error, want rejection", bad)
		}
	}
}

func TestParseTreatsBareCRAsLineBreak(t *testing.T) {
	doc := Parse("- a\r- b\r")
	if got := doc.Len(); got != 2 {
		t.Fatalf("Len = %d, want 2 (bare CR must not forge a single item)", got)
	}
	if strings.Contains(doc.String(), "\r") {
		t.Errorf("serialized output still has CR: %q", doc.String())
	}
}

func TestSelectExactRejectsSubstringOnly(t *testing.T) {
	doc := Parse("- deploy master\n- deploy web\n")
	if n, err := doc.SelectExact("1"); err != nil || n != 1 {
		t.Errorf("SelectExact(\"1\") = %d, %v; want 1, nil", n, err)
	}
	if n, err := doc.SelectExact("DEPLOY WEB"); err != nil || n != 2 {
		t.Errorf("SelectExact(exact text) = %d, %v; want 2, nil", n, err)
	}
	if _, err := doc.SelectExact("deploy"); err == nil {
		t.Error("SelectExact(substring) = nil error, want rejection")
	}
}
