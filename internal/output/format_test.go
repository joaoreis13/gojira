package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestApplyFieldsOnSearchEnvelope(t *testing.T) {
	var data any
	raw := `{"total":1,"issues":[{"key":"PROJ-1","fields":{"summary":"Fix bug","status":{"name":"Open"},"assignee":{"displayName":"Ada"}}}]}`
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}

	filtered := ApplyFields(data, []string{"key", "fields.summary", "fields.status.name"})

	out, err := json.Marshal(filtered)
	if err != nil {
		t.Fatal(err)
	}

	var got map[string]any
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatal(err)
	}
	issues, ok := got["issues"].([]any)
	if !ok || len(issues) != 1 {
		t.Fatalf("expected 1 issue in filtered output, got %#v", got)
	}
	issue := issues[0].(map[string]any)
	if _, ok := issue["total"]; ok {
		t.Errorf("unexpected 'total' leaked into per-issue object: %#v", issue)
	}
	fields := issue["fields"].(map[string]any)
	if fields["summary"] != "Fix bug" {
		t.Errorf("summary = %v, want %q", fields["summary"], "Fix bug")
	}
	if _, present := fields["assignee"]; present {
		t.Errorf("assignee should have been dropped by field selection, got %#v", fields)
	}
	status := fields["status"].(map[string]any)
	if status["name"] != "Open" {
		t.Errorf("status.name = %v, want %q", status["name"], "Open")
	}
}

func TestSummarizeIssueList(t *testing.T) {
	var data any
	raw := `{"issues":[{"key":"PROJ-1","fields":{"summary":"Fix bug","status":{"name":"Open"}}}]}`
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := Summarize(&buf, data); err != nil {
		t.Fatal(err)
	}
	line := strings.TrimSpace(buf.String())
	want := "PROJ-1\tOpen\tFix bug"
	if line != want {
		t.Errorf("Summarize() = %q, want %q", line, want)
	}
}

func TestSummarizeFallsBackToJSONForUnknownShape(t *testing.T) {
	var data any
	raw := `{"weird":"shape","nested":{"a":1}}`
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if err := Summarize(&buf, data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), `"weird":"shape"`) {
		t.Errorf("expected compact JSON fallback, got %q", buf.String())
	}
}
