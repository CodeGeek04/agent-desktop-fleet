package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/CodeGeek04/agent-desktop-fleet/skills"
)

// skillTarget is one skills folder and the coding agents that read it.
type skillTarget struct {
	agents []string // agents that read this folder, for messages
	dir    string   // the skills folder; the skill goes in dir/<skills.Name>/SKILL.md
}

// skillEnv is what detection needs from the environment, so tests can fake it.
type skillEnv struct {
	home   string
	getenv func(string) string
	exists func(string) bool
}

func realSkillEnv() (skillEnv, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return skillEnv{}, err
	}
	return skillEnv{home: home, getenv: os.Getenv, exists: func(p string) bool {
		_, err := os.Stat(p)
		return err == nil
	}}, nil
}

// orDefault returns the environment variable's value, or def when it is unset.
func (e skillEnv) orDefault(name, def string) string {
	if v := e.getenv(name); v != "" {
		return v
	}
	return def
}

// detectSkillTargets finds the coding agents installed for this user and the skills
// folders they read. Most agents read the shared ~/.agents/skills folder; Claude Code,
// Cline and Kiro each read their own.
func detectSkillTargets(e skillEnv) []skillTarget {
	h := func(parts ...string) string { return filepath.Join(append([]string{e.home}, parts...)...) }
	var out []skillTarget

	claude := e.orDefault("CLAUDE_CONFIG_DIR", h(".claude"))
	if e.exists(claude) {
		out = append(out, skillTarget{agents: []string{"Claude Code"}, dir: filepath.Join(claude, "skills")})
	}

	shared := []struct{ agent, marker string }{
		{"Codex", e.orDefault("CODEX_HOME", h(".codex"))},
		{"Cursor", h(".cursor")},
		{"Gemini CLI", h(".gemini")},
		{"GitHub Copilot", e.orDefault("COPILOT_HOME", h(".copilot"))},
		{"OpenCode", h(".config", "opencode")},
		{"Amp", h(".config", "amp")},
		{"Goose", h(".config", "goose")},
		{"Windsurf", h(".config", "devin")},
		{"Windsurf", h(".codeium", "windsurf")},
	}
	var sharedAgents []string
	for _, s := range shared {
		if e.exists(s.marker) && !contains(sharedAgents, s.agent) {
			sharedAgents = append(sharedAgents, s.agent)
		}
	}
	if len(sharedAgents) == 0 && e.exists(h(".agents", "skills")) {
		sharedAgents = []string{"agents that read ~/.agents/skills"}
	}
	if len(sharedAgents) > 0 {
		out = append(out, skillTarget{agents: sharedAgents, dir: h(".agents", "skills")})
	}

	for _, own := range []struct{ agent, dir string }{{"Cline", ".cline"}, {"Kiro", ".kiro"}} {
		if e.exists(h(own.dir)) {
			out = append(out, skillTarget{agents: []string{own.agent}, dir: h(own.dir, "skills")})
		}
	}
	return out
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// installSkill writes the skill into dir/<name>/SKILL.md and reports whether it changed.
func installSkill(dir string, content []byte) (changed bool, err error) {
	path := filepath.Join(dir, skills.Name, "SKILL.md")
	if old, err := os.ReadFile(path); err == nil && bytes.Equal(old, content) {
		return false, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	return true, os.WriteFile(path, content, 0o644)
}

// tilde shortens paths under the home folder for display.
func tilde(home, p string) string {
	if home != "" && strings.HasPrefix(p, home+string(filepath.Separator)) {
		return "~" + p[len(home):]
	}
	return p
}

// adfCommand is how agents should start adf: the name on PATH when it resolves there (so
// Homebrew upgrades keep working), otherwise this binary's own path.
func adfCommand() string {
	if p, err := exec.LookPath("adf"); err == nil {
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
	}
	if p, err := os.Executable(); err == nil {
		return p
	}
	return "adf"
}

// mcpHints returns how to register the adf MCP server with each detected agent.
func mcpHints(targets []skillTarget, adf string) []string {
	known := map[string]string{
		"Claude Code":    "claude mcp add --scope user adf -- " + adf + " mcp",
		"Codex":          "codex mcp add adf -- " + adf + " mcp",
		"Gemini CLI":     "gemini mcp add -s user adf " + adf + " mcp",
		"GitHub Copilot": "copilot mcp add adf -- " + adf + " mcp",
		"OpenCode":       "opencode mcp add adf --global -- " + adf + " mcp",
		"Windsurf":       "devin mcp add adf -s user -- " + adf + " mcp",
		"Cursor":         `add to ~/.cursor/mcp.json: {"mcpServers":{"adf":{"command":"` + adf + `","args":["mcp"]}}}`,
	}
	var out []string
	for _, t := range targets {
		for _, a := range t.agents {
			if cmd, ok := known[a]; ok {
				out = append(out, fmt.Sprintf("  %-15s %s", a+":", cmd))
			}
		}
	}
	return out
}

func skillCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "skill",
		Short: "Install the adf skill for your coding agents (Claude Code, Codex, Cursor, Gemini CLI and more)",
		Long: "The adf skill teaches a coding agent to split a task across desktops, stay within this computer's\n" +
			"limits, run desktops in parallel, hand control to you for logins or payments, and clean up.",
	}
	cmd.AddCommand(skillInstallCommand(), skillUninstallCommand(), skillShowCommand())
	return cmd
}

func skillInstallCommand() *cobra.Command {
	var dirs []string
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Copy the skill into the skills folder of every coding agent found for this user",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, err := realSkillEnv()
			if err != nil {
				return err
			}
			return runSkillInstall(cmd.OutOrStdout(), e, dirs, dryRun, adfCommand())
		},
	}
	cmd.Flags().StringArrayVar(&dirs, "dir", nil, "install into this skills folder instead of detecting agents (repeatable)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show where the skill would go without writing anything")
	return cmd
}

func runSkillInstall(w io.Writer, e skillEnv, dirs []string, dryRun bool, adf string) error {
	targets := detectSkillTargets(e)
	if len(dirs) > 0 {
		targets = nil
		for _, d := range dirs {
			targets = append(targets, skillTarget{agents: []string{"custom folder"}, dir: d})
		}
	}
	if len(targets) == 0 {
		fmt.Fprintln(w, "No coding agents found for this user (looked for Claude Code, Codex, Cursor, Gemini CLI,")
		fmt.Fprintln(w, "GitHub Copilot, OpenCode, Amp, Goose, Windsurf, Cline and Kiro).")
		fmt.Fprintln(w, "Install into a specific folder with: adf skill install --dir PATH")
		fmt.Fprintln(w, "Or print the skill with: adf skill show")
		return nil
	}
	for _, t := range targets {
		path := tilde(e.home, filepath.Join(t.dir, skills.Name, "SKILL.md"))
		who := strings.Join(t.agents, ", ")
		if dryRun {
			fmt.Fprintf(w, "would install for %s: %s\n", who, path)
			continue
		}
		changed, err := installSkill(t.dir, skills.Desktops)
		if err != nil {
			return fmt.Errorf("installing for %s: %w", who, err)
		}
		state := "installed"
		if !changed {
			state = "already up to date"
		}
		fmt.Fprintf(w, "%s for %s: %s\n", state, who, path)
	}
	if hints := mcpHints(targets, adf); len(hints) > 0 && len(dirs) == 0 && !dryRun {
		fmt.Fprintln(w, "\nThe skill uses the adf MCP tools. If you have not added them yet:")
		for _, h := range hints {
			fmt.Fprintln(w, h)
		}
	}
	return nil
}

func skillUninstallCommand() *cobra.Command {
	var dirs []string
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the skill from every coding agent's skills folder",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			e, err := realSkillEnv()
			if err != nil {
				return err
			}
			targets := detectSkillTargets(e)
			if len(dirs) > 0 {
				targets = nil
				for _, d := range dirs {
					targets = append(targets, skillTarget{agents: []string{"custom folder"}, dir: d})
				}
			}
			removed := 0
			for _, t := range targets {
				p := filepath.Join(t.dir, skills.Name)
				if _, err := os.Stat(filepath.Join(p, "SKILL.md")); err != nil {
					continue
				}
				if err := os.RemoveAll(p); err != nil {
					return err
				}
				removed++
				fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", tilde(e.home, p))
			}
			if removed == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "the adf skill is not installed in any skills folder found")
			}
			return nil
		},
	}
	cmd.Flags().StringArrayVar(&dirs, "dir", nil, "remove from this skills folder instead of detecting agents (repeatable)")
	return cmd
}

func skillShowCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "show",
		Short: "Print the skill (SKILL.md) to standard output",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := cmd.OutOrStdout().Write(skills.Desktops)
			return err
		},
	}
}
