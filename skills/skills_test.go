package skills

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestPluginCopyMatches(t *testing.T) {
	plugin, err := os.ReadFile("../plugin/skills/" + Name + "/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(plugin, Desktops) {
		t.Fatalf("plugin/skills/%s/SKILL.md differs from skills/%s/SKILL.md; copy the canonical file over", Name, Name)
	}
}

func TestFrontmatter(t *testing.T) {
	s := string(Desktops)
	if !strings.HasPrefix(s, "---\nname: "+Name+"\ndescription: ") {
		t.Fatalf("SKILL.md must start with frontmatter naming %q (the folder name) and a description", Name)
	}
	end := strings.Index(s[4:], "\n---\n")
	if end < 0 {
		t.Fatal("SKILL.md frontmatter is not closed")
	}
	for _, line := range strings.Split(s[4:4+end], "\n") {
		if strings.HasPrefix(line, "description: ") && len(line)-len("description: ") > 1024 {
			t.Fatalf("description is %d characters; keep it to 1024", len(line)-len("description: "))
		}
	}
}
