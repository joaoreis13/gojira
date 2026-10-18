package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/joaoreis13/gojira/internal/checklist"
	"github.com/joaoreis13/gojira/internal/client"
)

// --- test harness -----------------------------------------------------------

type recordedRequest struct {
	Method string
	Path   string
	Query  url.Values
	Body   string
}

type fakeJira struct {
	mu       sync.Mutex
	requests []recordedRequest
	routes   map[string]func(w http.ResponseWriter, r *http.Request, body string)
	out      *bytes.Buffer
}

func newFakeJira(t *testing.T) *fakeJira {
	t.Helper()
	f := &fakeJira{routes: map[string]func(http.ResponseWriter, *http.Request, string){}}
	return f
}

func (f *fakeJira) handle(method, path string, fn func(w http.ResponseWriter, r *http.Request, body string)) {
	f.routes[method+" "+path] = fn
}

func (f *fakeJira) start(t *testing.T) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.mu.Lock()
		f.requests = append(f.requests, recordedRequest{Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Body: string(body)})
		fn, ok := f.routes[r.Method+" "+r.URL.Path]
		f.mu.Unlock()
		if !ok {
			http.Error(w, `{"errorMessages":["unexpected request `+r.Method+` `+r.URL.Path+`"]}`, http.StatusNotFound)
			return
		}
		fn(w, r, string(body))
	}))
	t.Cleanup(srv.Close)

	orig := checklistClient
	checklistClient = func(ctx context.Context, alias string) (*client.Client, string, error) {
		return &client.Client{HTTP: srv.Client(), BaseURL: srv.URL + "/rest/api/3"}, "test", nil
	}
	t.Cleanup(func() { checklistClient = orig })

	origOut := checklistOut
	buf := &bytes.Buffer{}
	checklistOut = buf
	t.Cleanup(func() { checklistOut = origOut })

	f.out = buf
}

func (f *fakeJira) output() string { return f.out.String() }

func (f *fakeJira) calls(method, path string) []recordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []recordedRequest
	for _, r := range f.requests {
		if r.Method == method && r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

func jsonResp(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	io.WriteString(w, body)
}

const fieldsJSON = `[{"id":"customfield_10001","name":"Checklists","custom":true,"schema":{"type":"string"}},` +
	`{"id":"customfield_10002","name":"Smart Checklist Progress","custom":true,"schema":{"type":"string"}}]`

func issueWithChecklist(text string) string {
	b, _ := json.Marshal(map[string]any{"key": "PROJ-1", "fields": map[string]any{"customfield_10001": text}})
	return string(b)
}

func newOpts() checklistOpts { return checklistOpts{Profile: "test", Output: "text"} }

// --- get --------------------------------------------------------------------

func TestChecklistGetAutoResolvesFieldAndRendersText(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/field", func(w http.ResponseWriter, r *http.Request, _ string) { jsonResp(w, 200, fieldsJSON) })
	f.handle("GET", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, issueWithChecklist("- one\n+ two\n"))
	})
	f.start(t)

	if err := runChecklistGet(context.Background(), newOpts(), "PROJ-1"); err != nil {
		t.Fatal(err)
	}
	out := f.output()
	for _, want := range []string{"PROJ-1", "customfield_10001", "1. [ ] one", "2. [x] two", "1/2 done"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	gets := f.calls("GET", "/rest/api/3/issue/PROJ-1")
	if len(gets) != 1 {
		t.Fatalf("issue GET count = %d, want 1", len(gets))
	}
	if got := gets[0].Query.Get("fields"); got != "customfield_10001" {
		t.Errorf("fields query = %q, want customfield_10001", got)
	}
}

func TestChecklistGetJSONOutput(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, issueWithChecklist("- one\n+ two\n"))
	})
	f.start(t)

	opts := newOpts()
	opts.Output = "json"
	opts.Field = "customfield_10001"
	if err := runChecklistGet(context.Background(), opts, "PROJ-1"); err != nil {
		t.Fatal(err)
	}
	var got struct {
		Issue string `json:"issue"`
		Field struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"field"`
		Items []struct {
			Number int    `json:"number"`
			Status string `json:"status"`
			Text   string `json:"text"`
		} `json:"items"`
		Done  int `json:"done"`
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(f.output()), &got); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, f.output())
	}
	if got.Issue != "PROJ-1" || got.Total != 2 || got.Done != 1 {
		t.Errorf("unexpected payload: %+v", got)
	}
	if len(got.Items) != 2 || got.Items[0].Status != "todo" || got.Items[1].Status != "done" {
		t.Errorf("unexpected items: %+v", got.Items)
	}
	if got.Field.ID != "customfield_10001" {
		t.Errorf("field id = %q", got.Field.ID)
	}
}

func TestChecklistGetEmptyFieldRendersZeroItems(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, `{"key":"PROJ-1","fields":{"customfield_10001":null}}`)
	})
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_10001"
	if err := runChecklistGet(context.Background(), opts, "PROJ-1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.output(), "0/0 done") {
		t.Errorf("output = %q, want a 0/0 summary", f.output())
	}
}

func TestChecklistGetAmbiguousFieldFails(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/field", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, `[{"id":"customfield_1","name":"Checklists","custom":true},{"id":"customfield_2","name":"Checklists","custom":true}]`)
	})
	f.start(t)

	err := runChecklistGet(context.Background(), newOpts(), "PROJ-1")
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
	if !strings.Contains(err.Error(), "--field") {
		t.Errorf("error = %q, want it to suggest --field", err)
	}
}

func TestChecklistGetNoFieldFailsWithInstallHint(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/field", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, `[{"id":"customfield_10002","name":"Smart Checklist Progress","custom":true}]`)
	})
	f.start(t)

	err := runChecklistGet(context.Background(), newOpts(), "PROJ-1")
	if err == nil {
		t.Fatal("expected error when no Checklists field exists, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "smart checklist") {
		t.Errorf("error = %q, want a Smart Checklist hint", err)
	}
}

func TestChecklistGetExplicitFieldSkipsDiscovery(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, `{"key":"PROJ-1","fields":{"customfield_99999":"- one\n"}}`)
	})
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_99999"
	if err := runChecklistGet(context.Background(), opts, "PROJ-1"); err != nil {
		t.Fatal(err)
	}
	if n := len(f.calls("GET", "/rest/api/3/field")); n != 0 {
		t.Errorf("field discovery ran %d time(s) despite --field, want 0", n)
	}
}

func TestChecklistGetPropagatesHTTPError(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/issue/NOPE-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 404, `{"errorMessages":["Issue does not exist"]}`)
	})
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_10001"
	if err := runChecklistGet(context.Background(), opts, "NOPE-1"); err == nil {
		t.Fatal("expected HTTP error, got nil")
	}
}

// --- set --------------------------------------------------------------------

func TestChecklistSetPUTsMarkdownToField(t *testing.T) {
	f := newFakeJira(t)
	f.handle("PUT", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusNoContent)
	})
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_10001"
	if err := runChecklistSet(context.Background(), opts, "PROJ-1", "- one\n+ two\n"); err != nil {
		t.Fatal(err)
	}
	puts := f.calls("PUT", "/rest/api/3/issue/PROJ-1")
	if len(puts) != 1 {
		t.Fatalf("PUT count = %d, want 1", len(puts))
	}
	var payload struct {
		Fields map[string]any `json:"fields"`
	}
	if err := json.Unmarshal([]byte(puts[0].Body), &payload); err != nil {
		t.Fatalf("PUT body is not JSON: %v (%s)", err, puts[0].Body)
	}
	if got := payload.Fields["customfield_10001"]; got != "- one\n+ two\n" {
		t.Errorf("field value = %v, want the raw markdown", got)
	}
	if !strings.Contains(f.output(), "updated") {
		t.Errorf("output = %q, want an update confirmation", f.output())
	}
}

func TestChecklistSetDryRunDoesNotWrite(t *testing.T) {
	f := newFakeJira(t)
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_10001"
	opts.DryRun = true
	if err := runChecklistSet(context.Background(), opts, "PROJ-1", "- one\n"); err != nil {
		t.Fatal(err)
	}
	if n := len(f.calls("PUT", "/rest/api/3/issue/PROJ-1")); n != 0 {
		t.Errorf("dry run issued %d PUT(s), want 0", n)
	}
	if !strings.Contains(f.output(), "- one") {
		t.Errorf("dry run output = %q, want the would-be field value", f.output())
	}
}

func TestChecklistSetRejectsEmptyContent(t *testing.T) {
	f := newFakeJira(t)
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_10001"
	if err := runChecklistSet(context.Background(), opts, "PROJ-1", "   \n"); err == nil {
		t.Error("expected error for empty checklist content, got nil")
	}
}

// --- mutations --------------------------------------------------------------

func TestChecklistCheckReadModifyWrites(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, issueWithChecklist("- deploy master\n- deploy web\n"))
	})
	f.handle("PUT", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusNoContent)
	})
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_10001"
	if err := runChecklistStatus(context.Background(), opts, "PROJ-1", "deploy master", checklist.Done); err != nil {
		t.Fatal(err)
	}
	puts := f.calls("PUT", "/rest/api/3/issue/PROJ-1")
	if len(puts) != 1 {
		t.Fatalf("PUT count = %d, want 1", len(puts))
	}
	if !strings.Contains(puts[0].Body, `+ deploy master`) {
		t.Errorf("PUT body = %s, want the checked marker on the selected item", puts[0].Body)
	}
	if !strings.Contains(puts[0].Body, `- deploy web`) {
		t.Errorf("PUT body = %s, want the untouched sibling preserved", puts[0].Body)
	}
}

func TestChecklistAmbiguousSelectorDoesNotWrite(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, issueWithChecklist("- deploy master\n- deploy web\n"))
	})
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_10001"
	err := runChecklistStatus(context.Background(), opts, "PROJ-1", "deploy", checklist.Done)
	if err == nil {
		t.Fatal("expected ambiguity error, got nil")
	}
	if n := len(f.calls("PUT", "/rest/api/3/issue/PROJ-1")); n != 0 {
		t.Errorf("ambiguous selector issued %d PUT(s), want 0", n)
	}
	if !strings.Contains(err.Error(), "1") || !strings.Contains(err.Error(), "2") {
		t.Errorf("error = %q, want the matching item numbers listed", err)
	}
}

func TestChecklistAddAppendsAndWrites(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, issueWithChecklist("- one\n"))
	})
	f.handle("PUT", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusNoContent)
	})
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_10001"
	if err := runChecklistAdd(context.Background(), opts, "PROJ-1", "two"); err != nil {
		t.Fatal(err)
	}
	puts := f.calls("PUT", "/rest/api/3/issue/PROJ-1")
	if len(puts) != 1 {
		t.Fatalf("PUT count = %d, want 1", len(puts))
	}
	if !strings.Contains(puts[0].Body, `- two`) {
		t.Errorf("PUT body = %s, want the appended item", puts[0].Body)
	}
}

func TestChecklistRemoveDeletesItem(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, issueWithChecklist("- keep\n- drop\n"))
	})
	f.handle("PUT", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusNoContent)
	})
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_10001"
	if err := runChecklistRemove(context.Background(), opts, "PROJ-1", "2"); err != nil {
		t.Fatal(err)
	}
	puts := f.calls("PUT", "/rest/api/3/issue/PROJ-1")
	if len(puts) != 1 {
		t.Fatalf("PUT count = %d, want 1", len(puts))
	}
	if strings.Contains(puts[0].Body, "drop") {
		t.Errorf("PUT body = %s, want the removed item gone", puts[0].Body)
	}
	if !strings.Contains(puts[0].Body, "keep") {
		t.Errorf("PUT body = %s, want the sibling kept", puts[0].Body)
	}
}

// --- discovery / raw --------------------------------------------------------

func TestChecklistFieldsListsCandidates(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/field", func(w http.ResponseWriter, r *http.Request, _ string) { jsonResp(w, 200, fieldsJSON) })
	f.start(t)

	if err := runChecklistFields(context.Background(), newOpts()); err != nil {
		t.Fatal(err)
	}
	out := f.output()
	for _, want := range []string{"customfield_10001", "Checklists", "customfield_10002", "Smart Checklist Progress"} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}

func TestChecklistRawReadsIssueProperty(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/issue/PROJ-1/properties/com.railsware.SmartChecklist.checklist", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, `{"key":"com.railsware.SmartChecklist.checklist","value":{"checkItems":[{"name":"one","checked":false}]}}`)
	})
	f.start(t)

	if err := runChecklistRaw(context.Background(), newOpts(), "PROJ-1"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(f.output(), "checkItems") {
		t.Errorf("output = %q, want the raw property JSON", f.output())
	}
}

// --- hardening from the security review -------------------------------------

func TestChecklistGetMissingFieldKeyErrors(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, `{"key":"PROJ-1","fields":{}}`)
	})
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_10001"
	err := runChecklistGet(context.Background(), opts, "PROJ-1")
	if err == nil {
		t.Fatal("expected an error when the field is absent from the response, got nil")
	}
	if !strings.Contains(err.Error(), "customfield_10001") {
		t.Errorf("error = %q, want it to name the missing field", err)
	}
}

func TestChecklistWriteRejectsNonCustomFieldID(t *testing.T) {
	f := newFakeJira(t)
	f.start(t)

	opts := newOpts()
	opts.Field = "summary"
	err := runChecklistSet(context.Background(), opts, "PROJ-1", "- one\n")
	if err == nil {
		t.Fatal("expected rejection of a system field as the write target, got nil")
	}
	if n := len(f.calls("PUT", "/rest/api/3/issue/PROJ-1")); n != 0 {
		t.Errorf("write happened %d time(s) despite an invalid field, want 0", n)
	}
}

func TestChecklistWriteAutoDetectRequiresExactChecklistsField(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/field", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, `[{"id":"customfield_7","name":"Checklist Template","custom":true}]`)
	})
	f.start(t)

	err := runChecklistSet(context.Background(), newOpts(), "PROJ-1", "- one\n")
	if err == nil {
		t.Fatal("expected writes to refuse a loose fuzzy field match, got nil")
	}
	if !strings.Contains(err.Error(), "--field") {
		t.Errorf("error = %q, want it to suggest --field", err)
	}
}

func TestChecklistGetSanitizesTerminalEscapes(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, issueWithChecklist("- \x1b]0;pwned\aitem\n# \x1b[2Kfake\n"))
	})
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_10001"
	if err := runChecklistGet(context.Background(), opts, "PROJ-1"); err != nil {
		t.Fatal(err)
	}
	if strings.ContainsRune(f.output(), 0x1b) {
		t.Errorf("raw ESC reached the terminal: %q", f.output())
	}
	if !strings.Contains(f.output(), "item") {
		t.Errorf("output lost the item text: %q", f.output())
	}
}

func TestChecklistWriteAllowAnyFieldBypassesCustomFieldGuard(t *testing.T) {
	f := newFakeJira(t)
	f.handle("PUT", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		w.WriteHeader(http.StatusNoContent)
	})
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_424242"
	opts.AllowAnyField = true
	if err := runChecklistSet(context.Background(), opts, "PROJ-1", "- one\n"); err != nil {
		t.Fatal(err)
	}
	if n := len(f.calls("PUT", "/rest/api/3/issue/PROJ-1")); n != 1 {
		t.Errorf("PUT count = %d, want 1 with --allow-any-field", n)
	}
}

func TestChecklistRemoveRejectsSubstringSelector(t *testing.T) {
	f := newFakeJira(t)
	f.handle("GET", "/rest/api/3/issue/PROJ-1", func(w http.ResponseWriter, r *http.Request, _ string) {
		jsonResp(w, 200, issueWithChecklist("- deploy master\n- deploy web\n"))
	})
	f.start(t)

	opts := newOpts()
	opts.Field = "customfield_10001"
	err := runChecklistRemove(context.Background(), opts, "PROJ-1", "deploy")
	if err == nil {
		t.Fatal("expected removal by bare substring to be refused, got nil")
	}
	if n := len(f.calls("PUT", "/rest/api/3/issue/PROJ-1")); n != 0 {
		t.Errorf("removal wrote %d time(s) despite an inexact selector, want 0", n)
	}
}
