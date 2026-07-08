package process

import "strings"

// toolRule maps a token to a tool identity. Matching is conservative: we look
// at the process name and command line but never claim certainty.
type toolRule struct {
	token      string
	tool       string
	toolType   string // ai_agent | editor | runtime | package_manager | vcs | shell | container
	confidence float64
}

// rules are checked in order; process-name matches score higher than cmdline.
var rules = []toolRule{
	{"claude-code", "claude-code", "ai_agent", 0.9},
	{"claude", "claude", "ai_agent", 0.8},
	{"cursor", "cursor", "editor", 0.8},
	{"codex", "codex", "ai_agent", 0.8},
	{"copilot", "copilot", "ai_agent", 0.7},
	{"node", "node", "runtime", 0.4},
	{"npm", "npm", "package_manager", 0.6},
	{"npx", "npx", "package_manager", 0.6},
	{"pnpm", "pnpm", "package_manager", 0.6},
	{"yarn", "yarn", "package_manager", 0.6},
	{"python3", "python3", "runtime", 0.4},
	{"python", "python", "runtime", 0.4},
	{"go", "go", "runtime", 0.4},
	{"cargo", "cargo", "package_manager", 0.6},
	{"docker", "docker", "container", 0.6},
	{"git", "git", "vcs", 0.5},
	{"bash", "bash", "shell", 0.3},
	{"zsh", "zsh", "shell", 0.3},
	{"sh", "sh", "shell", 0.3},
}

// DetectTool inspects a process name and command line for a known AI/dev tool.
// It returns an empty tool string when nothing matches.
func DetectTool(name, cmdline string) (tool, toolType string, confidence float64, reason string) {
	lname := strings.ToLower(name)
	lcmd := strings.ToLower(cmdline)

	// The first whitespace-delimited word of the process name, so multi-word
	// names like "Cursor Helper (Renderer)" match on "cursor" but "Google
	// Chrome" does not spuriously match "go".
	firstWord := lname
	if i := strings.IndexByte(lname, ' '); i >= 0 {
		firstWord = lname[:i]
	}

	// Prefer an exact process-name (first-word) match.
	for _, r := range rules {
		if lname == r.token || firstWord == r.token {
			return r.tool, r.toolType, r.confidence, "process name matched " + r.token
		}
	}
	// Then a command-line token match, scored lower.
	for _, r := range rules {
		if containsToken(lcmd, r.token) {
			return r.tool, r.toolType, r.confidence * 0.7, "command line referenced " + r.token
		}
	}
	return "", "", 0, ""
}

// containsToken reports whether tok appears as a whitespace- or path-delimited
// token within s, avoiding substring false positives.
func containsToken(s, tok string) bool {
	idx := 0
	for {
		i := strings.Index(s[idx:], tok)
		if i < 0 {
			return false
		}
		start := idx + i
		end := start + len(tok)
		leftOK := start == 0 || isDelim(s[start-1])
		rightOK := end == len(s) || isDelim(s[end])
		if leftOK && rightOK {
			return true
		}
		idx = end
	}
}

func isDelim(b byte) bool {
	switch b {
	case ' ', '\t', '/', '\\', ':', '=', '"', '\'':
		return true
	}
	return false
}
