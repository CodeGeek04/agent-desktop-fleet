package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/CodeGeek04/agent-desktop-fleet/skills"
)

// fakeHome makes a home folder containing the given relative directories.
func fakeHome(t *testing.T, env map[string]string, dirs ...string) skillEnv {
	t.Helper()
	home := t.TempDir()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(home, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return skillEnv{
		home:   home,
		getenv: func(k string) string { return env[k] },
		exists: func(p string) bool { _, err := os.Stat(p); return err == nil },
	}
}

func TestDetectSkillTargets(t *testing.T) {
	e := fakeHome(t, nil, ".claude", ".codex", ".cursor", ".gemini", ".kiro")
	got := detectSkillTargets(e)
	if len(got) != 3 {
		t.Fatalf("got %d targets, want 3 (Claude Code, shared, Kiro): %+v", len(got), got)
	}
	if got[0].dir != filepath.Join(e.home, ".claude", "skills") || got[0].agents[0] != "Claude Code" {
		t.Errorf("first target = %+v, want Claude Code's skills folder", got[0])
	}
	if got[1].dir != filepath.Join(e.home, ".agents", "skills") || strings.Join(got[1].agents, ",") != "Codex,Cursor,Gemini CLI" {
		t.Errorf("shared target = %+v, want ~/.agents/skills for Codex, Cursor and Gemini CLI", got[1])
	}
	if got[2].dir != filepath.Join(e.home, ".kiro", "skills") {
		t.Errorf("third target = %+v, want Kiro's skills folder", got[2])
	}
}

func TestDetectSkillTargetsHonorsClaudeConfigDir(t *testing.T) {
	e := fakeHome(t, nil, "profiles/work")
	e.getenv = func(k string) string {
		if k == "CLAUDE_CONFIG_DIR" {
			return filepath.Join(e.home, "profiles/work")
		}
		return ""
	}
	got := detectSkillTargets(e)
	if len(got) != 1 || got[0].dir != filepath.Join(e.home, "profiles/work", "skills") {
		t.Fatalf("got %+v, want only the CLAUDE_CONFIG_DIR skills folder", got)
	}
}

func TestDetectSkillTargetsNone(t *testing.T) {
	if got := detectSkillTargets(fakeHome(t, nil)); len(got) != 0 {
		t.Fatalf("got %+v, want no targets in an empty home", got)
	}
}

func TestRunSkillInstallWritesAndIsIdempotent(t *testing.T) {
	e := fakeHome(t, nil, ".claude", ".codex")
	var out bytes.Buffer
	if err := runSkillInstall(&out, e, nil, false, "/usr/local/bin/adf"); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{".claude/skills", ".agents/skills"} {
		b, err := os.ReadFile(filepath.Join(e.home, dir, skills.Name, "SKILL.md"))
		if err != nil || !bytes.Equal(b, skills.Desktops) {
			t.Fatalf("%s: skill not written (%v)", dir, err)
		}
	}
	text := out.String()
	for _, want := range []string{"installed for Claude Code: ~/.claude/skills/adf-desktops/SKILL.md",
		"installed for Codex: ~/.agents/skills/adf-desktops/SKILL.md",
		"claude mcp add --scope user adf -- /usr/local/bin/adf mcp",
		"codex mcp add adf -- /usr/local/bin/adf mcp"} {
		if !strings.Contains(text, want) {
			t.Errorf("output missing %q:\n%s", want, text)
		}
	}
	out.Reset()
	if err := runSkillInstall(&out, e, nil, false, "adf"); err != nil {
		t.Fatal(err)
	}
	if strings.Count(out.String(), "already up to date") != 2 {
		t.Errorf("second install should report both copies up to date:\n%s", out.String())
	}
}

func TestRunSkillInstallDryRunWritesNothing(t *testing.T) {
	e := fakeHome(t, nil, ".claude")
	var out bytes.Buffer
	if err := runSkillInstall(&out, e, nil, true, "adf"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(e.home, ".claude", "skills", skills.Name)); !os.IsNotExist(err) {
		t.Fatalf("dry run wrote the skill (stat err %v)", err)
	}
	if !strings.Contains(out.String(), "would install for Claude Code") {
		t.Errorf("dry run output: %s", out.String())
	}
}

func TestRunSkillInstallCustomDir(t *testing.T) {
	e := fakeHome(t, nil, ".claude")
	dir := filepath.Join(t.TempDir(), "my-skills")
	var out bytes.Buffer
	if err := runSkillInstall(&out, e, []string{dir}, false, "adf"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, skills.Name, "SKILL.md")); err != nil {
		t.Fatalf("custom dir install: %v", err)
	}
	if _, err := os.Stat(filepath.Join(e.home, ".claude", "skills")); !os.IsNotExist(err) {
		t.Fatal("--dir should skip detected agents")
	}
}

func TestRunSkillInstallNoAgents(t *testing.T) {
	var out bytes.Buffer
	if err := runSkillInstall(&out, fakeHome(t, nil), nil, false, "adf"); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "No coding agents found") {
		t.Errorf("output: %s", out.String())
	}
}
