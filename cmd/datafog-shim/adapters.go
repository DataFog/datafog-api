package main

import (
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

type adapterSpec struct {
	Canonical   string
	Aliases     []string
	Description string
}

var defaultAdapters = []adapterSpec{
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
}

var adapterCanonicalByAlias = map[string]string{}

func init() {
	for _, spec := range defaultAdapters {
		adapterCanonicalByAlias[strings.ToLower(spec.Canonical)] = spec.Canonical
		for _, alias := range spec.Aliases {
			adapterCanonicalByAlias[strings.ToLower(alias)] = spec.Canonical
		}
	}
}

func resolveAdapter(raw string, command string) string {
	normalized := normalizeAdapter(raw)
	if normalized == "" {
		normalized = normalizeAdapter(command)
	}
	if normalized == "" {
		return ""
	}
	if canonical, ok := canonicalAdapter(normalized); ok {
		return canonical
	}
	return normalized
}

func canonicalAdapter(value string) (string, bool) {
	canonical, ok := adapterCanonicalByAlias[strings.ToLower(value)]
	return canonical, ok
}

func normalizeAdapter(raw string) string {
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

func knownAdapters() []adapterSpec {
	result := make([]adapterSpec, 0, len(defaultAdapters))
	result = append(result, defaultAdapters...)
	sort.Slice(result, func(i, j int) bool {
		return result[i].Canonical < result[j].Canonical
	})
	return result
}
