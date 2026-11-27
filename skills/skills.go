// Package skills embeds the agent skill that ships with adf, so `adf skill install` can
// copy it into each coding agent's skills folder. The Claude Code plugin carries an
// identical copy in plugin/skills; a test keeps the two in step.
package skills

import _ "embed"

// Name is the skill's name and the folder it is installed into.
const Name = "adf-desktops"

// Desktops is the SKILL.md for running adf desktops.
//
//go:embed adf-desktops/SKILL.md
var Desktops []byte
