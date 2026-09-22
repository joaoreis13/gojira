// Package output shapes decoded Jira API JSON for either machine
// consumption (compact/filtered JSON) or a token-cheap human/agent-readable
// summary, so callers don't have to ship full raw responses into an LLM
// context window.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// ApplyFields narrows data down to the given dot-paths (e.g.
// "fields.summary", "fields.status.name"). It understands Jira's common
// list envelopes ("issues": [...], "values": [...]) and applies the
// selection to each element rather than the envelope itself.
func ApplyFields(data any, fields []string) any {
	if len(fields) == 0 {
		return data
	}
	switch v := data.(type) {
	case []any:
		return selectEach(v, fields)
	case map[string]any:
		for _, envelopeKey := range []string{"issues", "values"} {
			if list, ok := v[envelopeKey].([]any); ok {
				out := make(map[string]any, len(v))
				for k, val := range v {
					if k == envelopeKey {
						out[k] = selectEach(list, fields)
					} else {
						out[k] = val
					}
				}
				return out
			}
		}
		return selectPaths(v, fields)
	default:
		return data
	}
}

func selectEach(items []any, fields []string) []any {
	out := make([]any, len(items))
	for i, item := range items {
		if obj, ok := item.(map[string]any); ok {
			out[i] = selectPaths(obj, fields)
		} else {
			out[i] = item
		}
	}
	return out
}

func selectPaths(obj map[string]any, fields []string) map[string]any {
	out := map[string]any{}
	for _, f := range fields {
		parts := strings.Split(f, ".")
		if val, ok := getPath(obj, parts); ok {
			setPath(out, parts, val)
		}
	}
	return out
}

func getPath(m map[string]any, parts []string) (any, bool) {
	cur := any(m)
	for _, p := range parts {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		val, ok := obj[p]
		if !ok {
			return nil, false
		}
		cur = val
	}
	return cur, true
}

func setPath(m map[string]any, parts []string, val any) {
	cur := m
	for i, p := range parts {
		if i == len(parts)-1 {
			cur[p] = val
			return
		}
		next, ok := cur[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			cur[p] = next
		}
		cur = next
	}
}

// Encode writes data as JSON, compact by default or indented when pretty.
func Encode(w io.Writer, data any, pretty bool) error {
	enc := json.NewEncoder(w)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(data)
}

// Summarize writes a tab-separated, line-per-item rendering for common Jira
// shapes (issue search results, a single issue, "values"-paginated admin
// lists). Anything it doesn't recognize falls back to compact JSON.
func Summarize(w io.Writer, data any) error {
	switch v := data.(type) {
	case map[string]any:
		if issues, ok := v["issues"].([]any); ok {
			for _, it := range issues {
				printIssueLine(w, it)
			}
			return nil
		}
		if _, ok := v["key"]; ok {
			printIssueLine(w, v)
			return nil
		}
		if values, ok := v["values"].([]any); ok {
			for _, it := range values {
				printGenericLine(w, it)
			}
			return nil
		}
	case []any:
		for _, it := range v {
			printGenericLine(w, it)
		}
		return nil
	}
	return Encode(w, data, false)
}

func printIssueLine(w io.Writer, item any) {
	obj, ok := item.(map[string]any)
	if !ok {
		printGenericLine(w, item)
		return
	}
	key, _ := obj["key"].(string)
	var summary, status string
	if fields, ok := obj["fields"].(map[string]any); ok {
		summary, _ = fields["summary"].(string)
		if st, ok := fields["status"].(map[string]any); ok {
			status, _ = st["name"].(string)
		}
	}
	fmt.Fprintf(w, "%s\t%s\t%s\n", key, status, summary)
}

func printGenericLine(w io.Writer, item any) {
	obj, ok := item.(map[string]any)
	if !ok {
		fmt.Fprintf(w, "%v\n", item)
		return
	}
	id := stringOrNumber(obj["id"])
	key, _ := obj["key"].(string)
	name, _ := obj["name"].(string)
	if name == "" {
		name, _ = obj["displayName"].(string)
	}
	switch {
	case key != "" && name != "":
		fmt.Fprintf(w, "%s\t%s\n", key, name)
	case id != "" && name != "":
		fmt.Fprintf(w, "%s\t%s\n", id, name)
	default:
		b, _ := json.Marshal(obj)
		fmt.Fprintln(w, string(b))
	}
}

func stringOrNumber(v any) string {
	switch n := v.(type) {
	case string:
		return n
	case float64:
		return strconv.FormatFloat(n, 'f', -1, 64)
	default:
		return ""
	}
}
