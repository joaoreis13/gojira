package cli

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/joaoreis13/gojira/internal/skillassets"
)

var (
	skillDestFlag    string
	skillProjectFlag bool
)

var skillCmd = &cobra.Command{
	Use:   "skill",
	Short: "Install the gojira Agent Skill so AI coding agents know how to use this CLI",
}

var skillInstallCmd = &cobra.Command{
	Use:   "install",
	Short: "Copy the bundled SKILL.md + reference docs onto this machine",
	Long: `Installs a portable "Agent Skill" (a SKILL.md plus reference docs,
in the format Claude Code and compatible agent harnesses discover skills
from) that teaches an AI agent how to check gojira's install/auth status,
call the API safely (including the destructive-action --yes gate), and
where to look up exact endpoint parameters, instead of it having to
rediscover all of that from scratch or from README.md alone.

By default this installs into your personal Claude Code skills directory
(~/.claude/skills/gojira). Use --project to install into the current
project's .claude/skills/gojira instead (so it's shared via version
control with collaborators), or --dest <dir> for any other location or
agent harness that reads skills from a directory of Markdown files.

Re-running this command overwrites a previous install in the same
location, which is expected when upgrading gojira.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		dest, err := resolveSkillDest()
		if err != nil {
			return err
		}
		n, err := installSkill(dest)
		if err != nil {
			return err
		}
		fmt.Printf("Installed gojira skill (%d files) to %s\n", n, dest)
		return nil
	},
}

func resolveSkillDest() (string, error) {
	if skillDestFlag != "" {
		return skillDestFlag, nil
	}
	if skillProjectFlag {
		return filepath.Join(".claude", "skills", "gojira"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "skills", "gojira"), nil
}

// installSkill copies the embedded skill tree into dest, returning the
// number of files written.
func installSkill(dest string) (int, error) {
	const root = "gojira"
	count := 0
	err := fs.WalkDir(skillassets.FS, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := fs.ReadFile(skillassets.FS, path)
		if err != nil {
			return fmt.Errorf("read embedded %s: %w", path, err)
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", target, err)
		}
		count++
		return nil
	})
	if err != nil {
		return 0, err
	}
	return count, nil
}

func init() {
	skillInstallCmd.Flags().StringVar(&skillDestFlag, "dest", "", "install into this directory instead of the default Claude Code skills location")
	skillInstallCmd.Flags().BoolVar(&skillProjectFlag, "project", false, "install into ./.claude/skills/gojira instead of the personal (~/.claude/skills/gojira) location")
	skillCmd.AddCommand(skillInstallCmd)
}
