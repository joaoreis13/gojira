// Package skillassets embeds the gojira Agent Skill (SKILL.md and its
// reference docs) into the binary so `gojira skill install` works from a
// plain `go install`, without needing a clone of the source repo around.
package skillassets

import "embed"

//go:embed gojira
var FS embed.FS
