package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestParseQuery(t *testing.T) {
	q, err := parseQuery([]string{"jql=project = PROJ", "maxResults=10"})
	if err != nil {
		t.Fatal(err)
	}
	if got := q.Get("jql"); got != "project = PROJ" {
		t.Errorf("jql = %q", got)
	}
	if got := q.Get("maxResults"); got != "10" {
		t.Errorf("maxResults = %q", got)
	}
}

func TestParseQueryRejectsMissingEquals(t *testing.T) {
	if _, err := parseQuery([]string{"no-equals-sign"}); err == nil {
		t.Error("expected error for malformed query pair, got nil")
	}
}

func TestReadBodyInline(t *testing.T) {
	b, err := readBody(`{"a":1}`)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"a":1}` {
		t.Errorf("readBody = %q", string(b))
	}
}

func TestReadBodyEmpty(t *testing.T) {
	b, err := readBody("")
	if err != nil {
		t.Fatal(err)
	}
	if b != nil {
		t.Errorf("expected nil body for empty --data, got %q", string(b))
	}
}

func TestReadBodyFile(t *testing.T) {
	f := t.TempDir() + "/body.json"
	if err := os.WriteFile(f, []byte(`{"from":"file"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	b, err := readBody("@" + f)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != `{"from":"file"}` {
		t.Errorf("readBody(@file) = %q", string(b))
	}
}

func TestRenderResponseAppliesFieldsAndText(t *testing.T) {
	var buf bytes.Buffer
	body := []byte(`{"issues":[{"key":"PROJ-1","fields":{"summary":"Fix it","status":{"name":"Open"}}}]}`)
	if err := renderResponse(&buf, body, "", "text", false); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "PROJ-1\tOpen\tFix it") {
		t.Errorf("renderResponse text output = %q", buf.String())
	}
}
