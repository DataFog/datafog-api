package adapters

import "testing"

func TestResolveClaudeAdapter(t *testing.T) {
	tests := []struct {
		raw      string
		command  string
		expected string
	}{
		{"claude", "", "claude"},
		{"claude-code", "", "claude"},
		{"claude-cli", "", "claude"},
		{"", "claude", "claude"},
	}
	for _, tt := range tests {
		got := Resolve(tt.raw, tt.command)
		if got != tt.expected {
			t.Errorf("Resolve(%q, %q) = %q, want %q", tt.raw, tt.command, got, tt.expected)
		}
	}
}

func TestResolveCodexAdapter(t *testing.T) {
	tests := []struct {
		raw      string
		command  string
		expected string
	}{
		{"codex", "", "codex"},
		{"openai-codex", "", "codex"},
		{"codex-cli", "", "codex"},
		{"", "codex", "codex"},
	}
	for _, tt := range tests {
		got := Resolve(tt.raw, tt.command)
		if got != tt.expected {
			t.Errorf("Resolve(%q, %q) = %q, want %q", tt.raw, tt.command, got, tt.expected)
		}
	}
}

func TestMatchesAdapter(t *testing.T) {
	tests := []struct {
		name    string
		tool    string
		targets []string
		want    bool
	}{
		{"empty targets matches anything", "git", nil, true},
		{"direct match", "claude", []string{"claude"}, true},
		{"alias match", "claude-code", []string{"claude"}, true},
		{"canonical match", "git", []string{"vcs"}, true},
		{"no match", "curl", []string{"claude", "codex"}, false},
		{"codex match", "codex-cli", []string{"codex"}, true},
		{"multiple targets", "claude", []string{"claude", "codex"}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchesAdapter(tt.tool, tt.targets)
			if got != tt.want {
				t.Errorf("MatchesAdapter(%q, %v) = %v, want %v", tt.tool, tt.targets, got, tt.want)
			}
		})
	}
}

func TestKnownAdaptersContainsClaudeAndCodex(t *testing.T) {
	all := KnownAdapters()
	foundClaude, foundCodex := false, false
	for _, a := range all {
		if a.Canonical == "claude" {
			foundClaude = true
		}
		if a.Canonical == "codex" {
			foundCodex = true
		}
	}
	if !foundClaude {
		t.Error("expected claude adapter in registry")
	}
	if !foundCodex {
		t.Error("expected codex adapter in registry")
	}
}
