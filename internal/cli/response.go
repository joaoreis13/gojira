package cli

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/joaoreis13/gojira/internal/output"
)

// readBody resolves a --data flag value: "" means no body, "-" means read
// stdin, a leading "@" means read that file path, anything else is treated
// as an inline JSON literal.
func readBody(data string) ([]byte, error) {
	switch {
	case data == "":
		return nil, nil
	case data == "-":
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			return nil, fmt.Errorf("read stdin: %w", err)
		}
		return b, nil
	case strings.HasPrefix(data, "@"):
		b, err := os.ReadFile(data[1:])
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", data[1:], err)
		}
		return b, nil
	default:
		return []byte(data), nil
	}
}

// parseQuery turns repeated "key=value" flags into url.Values.
func parseQuery(pairs []string) (url.Values, error) {
	q := url.Values{}
	for _, p := range pairs {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return nil, fmt.Errorf("invalid --query %q, expected key=value", p)
		}
		q.Add(k, v)
	}
	return q, nil
}

// renderResponse decodes a JSON response body, optionally narrows it with
// --fields, and writes it as JSON or a token-cheap text summary.
func renderResponse(w io.Writer, body []byte, fieldsFlag, outputMode string, pretty bool) error {
	if len(body) == 0 {
		return nil
	}
	var data any
	if err := json.Unmarshal(body, &data); err != nil {
		// Non-JSON responses (e.g. raw attachment bytes from
		// /attachment/content/{id}) are written verbatim: no appended
		// newline, so `gojira api GET ... > file` round-trips binary content
		// byte-for-byte.
		_, werr := w.Write(body)
		return werr
	}
	if fieldsFlag != "" {
		data = output.ApplyFields(data, strings.Split(fieldsFlag, ","))
	}
	if outputMode == "text" {
		return output.Summarize(w, data)
	}
	return output.Encode(w, data, pretty)
}

// confirmDestructive prompts on stderr/stdin. If stdin isn't interactive
// (e.g. an agent invocation with no TTY), it declines rather than hanging,
// so callers must pass --yes explicitly in non-interactive contexts.
func confirmDestructive(method, path string) bool {
	fmt.Fprintf(os.Stderr, "%s %s is a destructive call. Re-run with --yes, or type \"yes\" to continue: ", method, path)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		fmt.Fprintln(os.Stderr, "\nno confirmation received; aborting")
		return false
	}
	return strings.TrimSpace(strings.ToLower(line)) == "yes"
}
