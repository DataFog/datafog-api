package adapters

import (
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

// Spec describes a recognized adapter category with canonical name and aliases.
type Spec struct {
	Canonical   string
	Aliases     []string
	Description string
}

// DefaultAdapters is the built-in adapter registry.
var DefaultAdapters = []Spec{
	{
		Canonical:   "vcs",
		Aliases:     []string{"git", "gh", "gogcli", "hub", "mercurial", "hg", "svn", "bzr", "fossil", "stgit"},
		Description: "Version-control clients and helpers",
	},
	{
		Canonical:   "shell",
		Aliases:     []string{"sh", "bash", "zsh", "fish", "csh", "tcsh", "cmd", "powershell", "pwsh"},
		Description: "Shell and command interpreters",
	},
	{
		Canonical:   "container",
		Aliases:     []string{"docker", "podman", "nerdctl", "crictl", "buildah"},
		Description: "Container runtimes and tooling",
	},
	{
		Canonical:   "kubernetes",
		Aliases:     []string{"kubectl", "helm", "oc", "k9s"},
		Description: "Kubernetes and cluster control tooling",
	},
	{
		Canonical:   "cloud_aws",
		Aliases:     []string{"aws", "aws2", "sam", "cdk", "eksctl"},
		Description: "AWS CLIs and wrappers",
	},
	{
		Canonical:   "cloud_gcp",
		Aliases:     []string{"gcloud", "gsutil", "bq", "gke"},
		Description: "Google Cloud CLIs and wrappers",
	},
	{
		Canonical:   "cloud_azure",
		Aliases:     []string{"az", "azure"},
		Description: "Azure CLIs and wrappers",
	},
	{
		Canonical:   "package_manager",
		Aliases:     []string{"npm", "pnpm", "yarn", "pip", "pip3", "poetry", "cargo", "go", "mvn", "gradle", "ruby", "gem"},
		Description: "Package manager and language ecosystem CLIs",
	},
	{
		Canonical:   "database",
		Aliases:     []string{"psql", "mysql", "mariadb", "sqlite3", "mongo", "mongosh", "redis-cli"},
		Description: "Datastore and SQL/NoSQL command interfaces",
	},
	{
		Canonical:   "http",
		Aliases:     []string{"curl", "wget", "http", "https"},
		Description: "HTTP/API request tooling",
	},
	{
		Canonical:   "claude",
		Aliases:     []string{"claude", "claude-code", "claude-cli"},
		Description: "Anthropic Claude Code AI coding assistant",
	},
	{
		Canonical:   "codex",
		Aliases:     []string{"codex", "openai-codex", "codex-cli"},
		Description: "OpenAI Codex AI coding assistant",
	},
}

var canonicalByAlias = map[string]string{}

func init() {
	for _, spec := range DefaultAdapters {
		canonicalByAlias[strings.ToLower(spec.Canonical)] = spec.Canonical
		for _, alias := range spec.Aliases {
			canonicalByAlias[strings.ToLower(alias)] = spec.Canonical
		}
	}
}

// Resolve maps a raw adapter name or command to its canonical adapter name.
func Resolve(raw string, command string) string {
	normalized := Normalize(raw)
	if normalized == "" {
		normalized = Normalize(command)
	}
	if normalized == "" {
		return ""
	}
	if canonical, ok := Canonical(normalized); ok {
		return canonical
	}
	return normalized
}

// Canonical returns the canonical adapter name for a given value.
func Canonical(value string) (string, bool) {
	canonical, ok := canonicalByAlias[strings.ToLower(value)]
	return canonical, ok
}

// Normalize strips path, extension, and lowercases the adapter name.
func Normalize(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	raw = filepath.Base(raw)
	raw = strings.TrimSuffix(strings.ToLower(raw), ".exe")
	if runtime.GOOS == "windows" {
		raw = strings.TrimSuffix(strings.ToLower(raw), ".cmd")
		raw = strings.TrimSuffix(strings.ToLower(raw), ".bat")
	}
	return strings.TrimSpace(raw)
}

// KnownAdapters returns all registered adapters sorted by canonical name.
func KnownAdapters() []Spec {
	result := make([]Spec, 0, len(DefaultAdapters))
	result = append(result, DefaultAdapters...)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Canonical < result[j].Canonical
	})
	return result
}

// MatchesAdapter checks if a given tool/adapter name matches any of the
// target adapter names (checking both canonical names and aliases).
func MatchesAdapter(tool string, targets []string) bool {
	if len(targets) == 0 {
		return true
	}
	toolCanonical, _ := Canonical(strings.ToLower(tool))
	toolLower := strings.ToLower(tool)

	for _, target := range targets {
		targetLower := strings.ToLower(target)
		targetCanonical, _ := Canonical(targetLower)

		// Direct match
		if toolLower == targetLower {
			return true
		}
		// Canonical match
		if toolCanonical != "" && toolCanonical == targetCanonical {
			return true
		}
		if toolCanonical != "" && toolCanonical == targetLower {
			return true
		}
		if toolLower == targetCanonical {
			return true
		}
	}
	return false
}
