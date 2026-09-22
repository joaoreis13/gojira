package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallSkillWritesExpectedFiles(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "gojira-skill")

	n, err := installSkill(dest)
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Errorf("installSkill wrote %d files, want 2", n)
	}

	skillMD := filepath.Join(dest, "SKILL.md")
	data, err := os.ReadFile(skillMD)
	if err != nil {
		t.Fatalf("SKILL.md not written: %v", err)
	}
	if !strings.HasPrefix(string(data), "---\nname: gojira\n") {
		t.Errorf("SKILL.md missing expected frontmatter, got: %.60q", data)
	}

	if _, err := os.Stat(filepath.Join(dest, "references", "commands.md")); err != nil {
		t.Errorf("references/commands.md not written: %v", err)
	}
}

func TestInstallSkillOverwritesExistingInstall(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "gojira-skill")

	if _, err := installSkill(dest); err != nil {
		t.Fatal(err)
	}
	if _, err := installSkill(dest); err != nil {
		t.Fatalf("second install should succeed (overwrite), got: %v", err)
	}
}

func TestResolveSkillDest(t *testing.T) {
	t.Cleanup(func() {
		skillDestFlag = ""
		skillProjectFlag = false
	})

	skillDestFlag = "/custom/path"
	skillProjectFlag = false
	got, err := resolveSkillDest()
	if err != nil {
		t.Fatal(err)
	}
	if got != "/custom/path" {
		t.Errorf("resolveSkillDest with --dest = %q, want /custom/path", got)
	}

	skillDestFlag = ""
	skillProjectFlag = true
	got, err = resolveSkillDest()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(".claude", "skills", "gojira")
	if got != want {
		t.Errorf("resolveSkillDest with --project = %q, want %q", got, want)
	}
}
