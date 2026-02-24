package main

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/datafog/datafog-api/internal/shim"
)

func TestParseMode(t *testing.T) {
	t.Run("default", func(t *testing.T) {
		mode, err := parseMode("")
		if err != nil {
			t.Fatalf("expected parse success, got %v", err)
		}
		if mode != shim.ModeEnforced {
			t.Fatalf("expected default enforced, got %q", mode)
		}
	})

	t.Run("observe", func(t *testing.T) {
		mode, err := parseMode("observe")
		if err != nil {
			t.Fatalf("expected parse success, got %v", err)
		}
		if mode != shim.ModeObserve {
			t.Fatalf("expected observe, got %q", mode)
		}
	})

	t.Run("invalid", func(t *testing.T) {
		if _, err := parseMode("invalid"); err == nil {
			t.Fatal("expected parse error")
		}
	})
}

func TestResolveRuntimeConfig(t *testing.T) {
	t.Setenv("DATAFOG_SHIM_POLICY_URL", "http://env:8080")
	t.Setenv("DATAFOG_SHIM_MODE", string(shim.ModeObserve))

	cfg, err := resolveRuntimeConfig(shimRuntimeConfig{})
	if err != nil {
		t.Fatalf("expected config resolve, got %v", err)
	}
	if cfg.policyURL != "http://env:8080" {
		t.Fatalf("expected env policy URL, got %q", cfg.policyURL)
	}
	if cfg.mode != string(shim.ModeObserve) {
		t.Fatalf("expected observe mode, got %q", cfg.mode)
	}
	if cfg.shimDir == "" {
		t.Fatal("expected shim directory fallback")
	}
}

func TestResolveTargetBinary(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "tool")
	if err := os.WriteFile(bin, []byte(""), 0o755); err != nil {
		t.Fatalf("write file: %v", err)
	}

	got, err := resolveTargetBinary(bin)
	if err != nil {
		t.Fatalf("expected absolute resolve, got %v", err)
	}
	if got != bin {
		abs, _ := filepath.Abs(bin)
		if got != abs {
			t.Fatalf("unexpected resolved path: %q", got)
		}
	}

	t.Run("pathLookup", func(t *testing.T) {
		path := t.TempDir()
		name := "lookupme"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		commandBin := filepath.Join(path, name)
		if err := os.WriteFile(commandBin, []byte(""), 0o755); err != nil {
			t.Fatalf("write path command: %v", err)
		}
		t.Setenv("PATH", path+string(filepath.ListSeparator)+os.Getenv("PATH"))

		got, err := resolveTargetBinary("lookupme")
		if err != nil {
			t.Fatalf("expected path resolve, got %v", err)
		}
		if got != commandBin && got != filepath.Clean(commandBin) {
			t.Fatalf("unexpected target resolve result: %q", got)
		}
	})
}

func TestBuildShimScript(t *testing.T) {
	script := buildShimScript(
		"/opt/datafog/datafog-shim",
		"git",
		"git",
		"/usr/bin/git",
		string(shim.ModeObserve),
		"http://localhost:8080",
		"/tmp/events.ndjson",
	)
	if !strings.Contains(script, shimMarker) {
		t.Fatalf("script missing shim marker")
	}
	if !strings.Contains(script, "# DATAFOG_SHIM_ADAPTER=git") {
		t.Fatalf("script missing adapter metadata")
	}
	if !strings.Contains(script, "# DATAFOG_SHIM_TARGET=/usr/bin/git") {
		t.Fatalf("script missing target metadata")
	}
	if !strings.Contains(script, `--mode "$SHIM_MODE"`) {
		t.Fatalf("script missing runtime mode wiring")
	}
}

func TestInstallListAndUninstallShim(t *testing.T) {
	root := t.TempDir()
	shimDir := filepath.Join(root, "shims")
	targetDir := filepath.Join(root, "targets")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		t.Fatalf("mkdir targets: %v", err)
	}

	targetBinary := filepath.Join(targetDir, "git")
	if err := os.WriteFile(targetBinary, []byte(""), 0o755); err != nil {
		t.Fatalf("write target binary: %v", err)
	}
	fakeShimBinary := filepath.Join(root, "datafog-shim")
	if err := os.WriteFile(fakeShimBinary, []byte("#!/bin/sh\necho shim\n"), 0o755); err != nil {
		t.Fatalf("write shim binary: %v", err)
	}

	cfg := shimRuntimeConfig{
		policyURL: "http://localhost:8080",
		mode:      string(shim.ModeEnforced),
		shimDir:   shimDir,
	}

	shimPath, err := installShimScript(fakeShimBinary, cfg, "git", "git", targetBinary, false)
	if err != nil {
		t.Fatalf("install shim failed: %v", err)
	}

	shimPath = filepath.Clean(shimPath)
	if runtime.GOOS != "windows" {
		if got := shimPath; got != filepath.Clean(shimScriptPath(shimDir, "git")) {
			t.Fatalf("unexpected shim path %q", got)
		}
	}

	found, managed, err := readShimMetadata(shimPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if !managed {
		t.Fatal("expected managed shim")
	}
	if found.Adapter != "vcs" {
		t.Fatalf("expected adapter vcs, got %q", found.Adapter)
	}

	list, err := listManagedShims(shimDir)
	if err != nil {
		t.Fatalf("list managed shims: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected one managed shim, got %d", len(list))
	}
	if list[0].Command != "git" {
		t.Fatalf("expected listed command git, got %q", list[0].Command)
	}

	uninstallCfg := cfg
	uninstallCfg.shimDir = shimDir
	if err := runHooksUninstall(uninstallCfg, []string{"git"}); err != nil {
		t.Fatalf("uninstall shim: %v", err)
	}

	if _, statErr := os.Stat(shimPath); !os.IsNotExist(statErr) {
		t.Fatalf("expected shim removed")
	}
}

func TestAdapterResolution(t *testing.T) {
	if adapter := resolveAdapter("", "/usr/bin/git"); adapter != "vcs" {
		t.Fatalf("expected git to resolve to vcs, got %q", adapter)
	}
	if adapter := resolveAdapter("", "/usr/local/bin/git"); adapter != "vcs" {
		t.Fatalf("expected absolute git path to resolve to vcs, got %q", adapter)
	}
	if adapter := resolveAdapter("", "/usr/bin/docker"); adapter != "container" {
		t.Fatalf("expected docker to resolve to container, got %q", adapter)
	}
	if adapter := resolveAdapter("  Git  ", ""); adapter != "vcs" {
		t.Fatalf("expected explicit git alias to resolve to vcs, got %q", adapter)
	}
	if adapter := resolveAdapter("  gh  ", ""); adapter != "vcs" {
		t.Fatalf("expected explicit gh alias to resolve to vcs, got %q", adapter)
	}
	if adapter := resolveAdapter("  gogcli  ", ""); adapter != "vcs" {
		t.Fatalf("expected explicit gogcli alias to resolve to vcs, got %q", adapter)
	}
	if adapter := resolveAdapter("customTool", "/usr/bin/git"); adapter != "customtool" {
		t.Fatalf("expected unknown adapter to normalize only, got %q", adapter)
	}
	if adapter := resolveAdapter("", "/usr/bin/customcommand"); adapter != "customcommand" {
		t.Fatalf("expected unknown command path to normalize only, got %q", adapter)
	}
}

func TestKnownAdaptersAreDeterministic(t *testing.T) {
	adapters := knownAdapters()
	if len(adapters) == 0 {
		t.Fatalf("expected known adapters list to be populated")
	}

	foundGit := false
	for _, adapter := range adapters {
		if adapter.Canonical == "vcs" {
			foundGit = true
			break
		}
	}
	if !foundGit {
		t.Fatalf("expected vcs canonical adapter")
	}
}
