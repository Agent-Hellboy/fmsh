package process

import "testing"

func TestDetectKnownTools(t *testing.T) {
	cases := map[string]string{
		"claude":      "claude",
		"claude-code": "claude-code",
		"cursor":      "cursor",
		"python3":     "python3",
		"node":        "node",
	}
	for name, want := range cases {
		tool, _, _, _ := DetectTool(name, "")
		if tool != want {
			t.Errorf("DetectTool(%q) = %q, want %q", name, tool, want)
		}
	}
}

func TestDetectMultiWordName(t *testing.T) {
	tool, _, _, _ := DetectTool("Cursor Helper (Renderer)", "")
	if tool != "cursor" {
		t.Errorf("multi-word Cursor name = %q, want cursor", tool)
	}
}

func TestNoFalsePositiveGoogle(t *testing.T) {
	// "Google Chrome" must not match the "go" tool.
	tool, _, _, _ := DetectTool("Google Chrome", "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome")
	if tool == "go" {
		t.Errorf("Google Chrome should not be detected as go")
	}
}

func TestDetectUnknown(t *testing.T) {
	if tool, _, _, _ := DetectTool("mdworker_shared", ""); tool != "" {
		t.Errorf("system process should not match a tool, got %q", tool)
	}
}

func TestRedactSecrets(t *testing.T) {
	in := "deploy --token=abc123 --api_key=SECRET password=hunter2"
	out := Redact(in)
	for _, leaked := range []string{"abc123", "SECRET", "hunter2"} {
		if contains(out, leaked) {
			t.Errorf("Redact left %q in %q", leaked, out)
		}
	}
	if !contains(out, "token=") {
		t.Errorf("Redact should preserve keys, got %q", out)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
