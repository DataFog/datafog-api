package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/datafog/datafog-api/internal/shim"
)

const (
	defaultPolicyURL = "http://localhost:8080"
	shimMarker       = "# datafog-shim-wrapper"
	shimMetaPrefix   = "# DATAFOG_SHIM_"
)

type shimRuntimeConfig struct {
	policyURL string
	apiToken  string
	mode      string
	eventSink string
	shimDir   string
	sensitive bool
}

type managedShimMetadata struct {
	Command   string
	Adapter   string
	Target    string
	Mode      string
	PolicyURL string
	EventSink string
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(argv []string) error {
	flags := flag.NewFlagSet("datafog-shim", flag.ContinueOnError)
	policyURL := flags.String("policy-url", "", "base URL for datafog API (for example http://localhost:8080)")
	apiToken := flags.String("api-token", "", "API token for policy decisions")
	mode := flags.String("mode", "", "enforcement mode: enforced|observe")
	eventSink := flags.String("event-sink", "", "path for NDJSON decision event sink")
	shimDir := flags.String("shim-dir", "", "directory for installed adapter shims")
	sensitive := flags.Bool("sensitive", false, "mark shimmed actions as sensitive")
	if err := flags.Parse(argv); err != nil {
		return err
	}

	cfg, err := resolveRuntimeConfig(shimRuntimeConfig{
		policyURL: *policyURL,
		apiToken:  *apiToken,
		mode:      *mode,
		eventSink: *eventSink,
		shimDir:   *shimDir,
		sensitive: *sensitive,
	})
	if err != nil {
		return err
	}

	args := flags.Args()
	if len(args) == 0 {
		return fmt.Errorf("missing command: hooks|shell|run|read-file|write-file\n\n%s", usage())
	}

	ctx := context.Background()
	cmd := args[0]
	switch cmd {
	case "shell":
		return runShell(ctx, cfg, args[1:])
	case "read-file":
		return runReadFile(ctx, cfg, args[1:])
	case "write-file":
		return runWriteFile(ctx, cfg, args[1:])
	case "run":
		return runCommandAdapter(ctx, cfg, args[1:])
	case "hooks":
		return runHooks(ctx, cfg, args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, usage())
	}
}

func resolveRuntimeConfig(input shimRuntimeConfig) (shimRuntimeConfig, error) {
	cfg := shimRuntimeConfig{
		policyURL: coalesce(input.policyURL, os.Getenv("DATAFOG_SHIM_POLICY_URL"), defaultPolicyURL),
		apiToken:  coalesce(input.apiToken, os.Getenv("DATAFOG_SHIM_API_TOKEN")),
		mode:      coalesce(input.mode, os.Getenv("DATAFOG_SHIM_MODE"), string(shim.ModeEnforced)),
		eventSink: coalesce(input.eventSink, os.Getenv("DATAFOG_SHIM_EVENT_SINK")),
		shimDir:   coalesce(input.shimDir, os.Getenv("DATAFOG_SHIM_DIR"), defaultShimDir()),
		sensitive: input.sensitive,
	}

	parsedMode, err := parseMode(cfg.mode)
	if err != nil {
		return shimRuntimeConfig{}, err
	}
	cfg.mode = string(parsedMode)
	return cfg, nil
}

func parseMode(raw string) (shim.EnforcementMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(shim.ModeEnforced):
		return shim.ModeEnforced, nil
	case string(shim.ModeObserve):
		return shim.ModeObserve, nil
	default:
		return "", fmt.Errorf("invalid mode %q (expected enforced or observe)", raw)
	}
}

func coalesce(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func newGate(cfg shimRuntimeConfig) *shim.Gate {
	client := shim.NewHTTPDecisionClient(cfg.policyURL, cfg.apiToken)
	mode := shim.ModeEnforced
	if m, err := parseMode(cfg.mode); err == nil {
		mode = m
	}
	opts := []shim.GateOption{
		shim.WithMode(mode),
	}
	if strings.TrimSpace(cfg.eventSink) != "" {
		opts = append(opts, shim.WithEventSink(shim.NewNDJSONDecisionEventSink(cfg.eventSink)))
	}
	return shim.NewGate(client, opts...)
}

func runShell(ctx context.Context, cfg shimRuntimeConfig, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("shell command is required")
	}
	command := args[0]
	shellArgs := args[1:]
	gate := newGate(cfg)
	decision, output, err := gate.ExecuteShell(ctx, command, shellArgs, "", nil, cfg.sensitive)
	if err != nil {
		return err
	}
	if len(output) > 0 {
		if _, writeErr := os.Stdout.Write(output); writeErr != nil {
			return writeErr
		}
	}
	if decision.ReceiptID != "" {
		fmt.Fprintf(os.Stderr, "receipt=%s decision=%s\n", decision.ReceiptID, decision.Decision)
	}
	return nil
}

func runReadFile(ctx context.Context, cfg shimRuntimeConfig, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("read-file path is required")
	}
	path := args[0]
	gate := newGate(cfg)
	decision, data, err := gate.ReadFile(ctx, path, "", nil, cfg.sensitive)
	if err != nil {
		return err
	}
	if _, writeErr := os.Stdout.Write(data); writeErr != nil {
		return writeErr
	}
	if !strings.HasSuffix(string(data), "\n") {
		if _, writeErr := os.Stdout.Write([]byte("\n")); writeErr != nil {
			return writeErr
		}
	}
	if decision.ReceiptID != "" {
		fmt.Fprintf(os.Stderr, "receipt=%s decision=%s\n", decision.ReceiptID, decision.Decision)
	}
	return nil
}

func runWriteFile(ctx context.Context, cfg shimRuntimeConfig, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("write-file requires <path> <text>")
	}
	path := args[0]
	content := strings.Join(args[1:], " ")
	gate := newGate(cfg)
	decision, err := gate.WriteFile(ctx, path, []byte(content), 0o600, "", nil, cfg.sensitive)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %d bytes to %s receipt=%s decision=%s\n", len(content), path, decision.ReceiptID, decision.Decision)
	return nil
}

func runCommandAdapter(ctx context.Context, cfg shimRuntimeConfig, args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	adapter := flags.String("adapter", "", "tool adapter name")
	target := flags.String("target", "", "binary or command to execute")
	overrideMode := flags.String("mode", "", "enforcement mode: enforced|observe")
	policyURL := flags.String("policy-url", "", "base URL for datafog API (for example http://localhost:8080)")
	apiToken := flags.String("api-token", "", "API token for policy decisions")
	eventSink := flags.String("event-sink", "", "path for NDJSON decision event sink")
	sensitive := flags.Bool("sensitive", false, "mark this action as sensitive")
	if err := flags.Parse(args); err != nil {
		return err
	}

	var err error
	cfg, err = resolveRuntimeConfig(shimRuntimeConfig{
		policyURL: coalesce(*policyURL, cfg.policyURL),
		apiToken:  coalesce(*apiToken, cfg.apiToken),
		mode:      coalesce(*overrideMode, cfg.mode),
		eventSink: coalesce(*eventSink, cfg.eventSink),
		shimDir:   cfg.shimDir,
		sensitive: *sensitive || cfg.sensitive,
	})
	if err != nil {
		return err
	}

	if strings.TrimSpace(*adapter) == "" {
		return fmt.Errorf("run requires --adapter")
	}
	runArgs := flags.Args()
	targetPath := strings.TrimSpace(*target)
	if targetPath == "" {
		if len(runArgs) == 0 {
			return fmt.Errorf("run requires --target <path> or a command")
		}
		targetPath = runArgs[0]
		runArgs = runArgs[1:]
	}
	if targetPath == "" {
		return fmt.Errorf("run target is required")
	}

	gate := newGate(cfg)
	decision, output, err := gate.ExecuteCommand(ctx, *adapter, targetPath, runArgs, "", nil, cfg.sensitive)
	if err != nil {
		return err
	}
	if len(output) > 0 {
		if _, writeErr := os.Stdout.Write(output); writeErr != nil {
			return writeErr
		}
	}
	if decision.ReceiptID != "" {
		fmt.Fprintf(os.Stderr, "receipt=%s decision=%s\n", decision.ReceiptID, decision.Decision)
	}
	return nil
}

func runHooks(_ context.Context, cfg shimRuntimeConfig, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing hooks subcommand: install|list|uninstall\n\n%s", usage())
	}
	switch args[0] {
	case "install":
		return runHooksInstall(cfg, args[1:])
	case "list":
		return runHooksList(cfg, args[1:])
	case "uninstall":
		return runHooksUninstall(cfg, args[1:])
	default:
		return fmt.Errorf("unknown hooks subcommand %q\n\n%s", args[0], usage())
	}
}

func runHooksInstall(cfg shimRuntimeConfig, argv []string) error {
	flags := flag.NewFlagSet("hooks install", flag.ContinueOnError)
	adapter := flags.String("adapter", "", "adapter name to report in policy")
	target := flags.String("target", "", "binary path to wrap (resolved from command if omitted)")
	force := flags.Bool("force", false, "overwrite unmanaged shim at same name")
	overrideMode := flags.String("mode", "", "override enforcement mode for this shim")
	overridePolicyURL := flags.String("policy-url", "", "override policy URL for this shim")
	overrideEventSink := flags.String("event-sink", "", "override event sink path for this shim")
	shimDir := flags.String("shim-dir", "", "directory for generated shim")
	if err := flags.Parse(argv); err != nil {
		return err
	}

	args := flags.Args()
	if len(args) != 1 {
		return fmt.Errorf("hooks install expects one command name")
	}

	installCfg := cfg
	installCfg.mode = coalesce(*overrideMode, cfg.mode)
	installCfg.policyURL = coalesce(*overridePolicyURL, cfg.policyURL)
	installCfg.eventSink = coalesce(*overrideEventSink, cfg.eventSink)
	if *shimDir != "" {
		installCfg.shimDir = *shimDir
	}
	installCfg.shimDir = filepath.Clean(installCfg.shimDir)
	if installCfg.shimDir == "" {
		return fmt.Errorf("shim directory is required")
	}

	command := strings.TrimSpace(args[0])
	if command == "" {
		return fmt.Errorf("command name is required")
	}
	adapterName := strings.TrimSpace(*adapter)
	if adapterName == "" {
		adapterName = command
	}

	targetPath := strings.TrimSpace(*target)
	if targetPath == "" {
		targetPath = command
	}
	resolvedTarget, err := resolveTargetBinary(targetPath)
	if err != nil {
		return err
	}

	shimBinary, err := os.Executable()
	if err != nil {
		return fmt.Errorf("unable to locate datafog-shim executable: %w", err)
	}
	shimBinary, err = filepath.Abs(shimBinary)
	if err != nil {
		return fmt.Errorf("unable to resolve shim binary path: %w", err)
	}

	shimPath, err := installShimScript(shimBinary, installCfg, command, adapterName, resolvedTarget, *force)
	if err != nil {
		return err
	}
	fmt.Printf("installed shim for %s at %s\n", command, shimPath)
	return nil
}

func runHooksList(cfg shimRuntimeConfig, argv []string) error {
	flags := flag.NewFlagSet("hooks list", flag.ContinueOnError)
	shimDir := flags.String("shim-dir", "", "directory for generated shims")
	if err := flags.Parse(argv); err != nil {
		return err
	}
	if *shimDir != "" {
		cfg.shimDir = *shimDir
	}
	cfg.shimDir = filepath.Clean(cfg.shimDir)
	if cfg.shimDir == "" {
		return fmt.Errorf("shim directory is required")
	}

	shims, err := listManagedShims(cfg.shimDir)
	if err != nil {
		return err
	}
	if len(shims) == 0 {
		fmt.Printf("no managed shims found in %s\n", cfg.shimDir)
		return nil
	}
	for _, m := range shims {
		fmt.Printf("%s -> target=%s adapter=%s mode=%s policy=%s\n", m.Command, m.Target, m.Adapter, m.Mode, m.PolicyURL)
	}
	return nil
}

func runHooksUninstall(cfg shimRuntimeConfig, argv []string) error {
	flags := flag.NewFlagSet("hooks uninstall", flag.ContinueOnError)
	shimDir := flags.String("shim-dir", "", "directory for generated shims")
	force := flags.Bool("force", false, "remove unmanaged file if name matches command")
	if err := flags.Parse(argv); err != nil {
		return err
	}
	if *shimDir != "" {
		cfg.shimDir = *shimDir
	}
	cfg.shimDir = filepath.Clean(cfg.shimDir)
	args := flags.Args()
	if len(args) != 1 {
		return fmt.Errorf("hooks uninstall expects one command name")
	}
	command := strings.TrimSpace(args[0])
	if command == "" {
		return fmt.Errorf("command name is required")
	}
	shimPath := shimScriptPath(cfg.shimDir, command)
	_, managed, err := readShimMetadata(shimPath)
	if err != nil {
		return err
	}
	if !managed && !*force {
		return fmt.Errorf("file %s is not a managed shim; use --force to remove", shimPath)
	}
	return os.Remove(shimPath)
}

func defaultShimDir() string {
	if override := strings.TrimSpace(os.Getenv("DATAFOG_SHIM_DIR")); override != "" {
		return override
	}
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE")
	}
	if home == "" {
		return filepath.Join(os.TempDir(), "datafog-shims")
	}
	return filepath.Join(home, ".datafog", "shims")
}

func resolveTargetBinary(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("target binary is required")
	}
	if strings.ContainsRune(raw, filepath.Separator) || strings.HasPrefix(raw, ".") {
		abs, err := filepath.Abs(raw)
		if err != nil {
			return "", fmt.Errorf("resolve target path %q: %w", raw, err)
		}
		info, err := os.Stat(abs)
		if err != nil {
			return "", fmt.Errorf("target binary not found: %w", err)
		}
		if info.IsDir() {
			return "", fmt.Errorf("target binary cannot be a directory: %s", abs)
		}
		return abs, nil
	}
	if runtime.GOOS == "windows" && !strings.ContainsRune(raw, filepath.Separator) && !strings.HasSuffix(strings.ToLower(raw), ".exe") {
		raw = raw + ".exe"
	}
	bin, err := exec.LookPath(raw)
	if err != nil {
		return "", fmt.Errorf("target binary not found %q: %w", raw, err)
	}
	return filepath.Clean(bin), nil
}

func installShimScript(shimBinary string, cfg shimRuntimeConfig, command, adapter, target string, force bool) (string, error) {
	cfg.mode = coalesce(cfg.mode, string(shim.ModeEnforced))
	cfg.shimDir = coalesce(cfg.shimDir, defaultShimDir())
	shimPath := shimScriptPath(cfg.shimDir, command)

	mode, err := parseMode(cfg.mode)
	if err != nil {
		return "", err
	}

	if mode == "" {
		mode = shim.ModeEnforced
	}

	if err := os.MkdirAll(cfg.shimDir, 0o755); err != nil {
		return "", fmt.Errorf("create shim directory %q: %w", cfg.shimDir, err)
	}

	metadata, managed, err := readShimMetadata(shimPath)
	if err != nil && !os.IsNotExist(err) {
		return "", err
	}
	if err == nil && !managed && !force {
		return "", fmt.Errorf("cannot overwrite unmanaged file %s; use --force", shimPath)
	}
	if metadata.Command == command && metadata.Adapter == adapter && metadata.Target == target && metadata.Mode == string(mode) {
		// idempotent overwrite allowed
	}

	content := buildShimScript(
		shimBinary,
		command,
		adapter,
		target,
		string(mode),
		cfg.policyURL,
		cfg.eventSink,
	)

	if err := os.WriteFile(shimPath, []byte(content), 0o755); err != nil {
		return "", fmt.Errorf("write shim %q: %w", shimPath, err)
	}
	return shimPath, nil
}

func listManagedShims(dir string) ([]managedShimMetadata, error) {
	dir = filepath.Clean(dir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	managed := make([]managedShimMetadata, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		meta, isManaged, err := readShimMetadata(filepath.Join(dir, entry.Name()))
		if err != nil {
			return nil, err
		}
		if !isManaged {
			continue
		}
		if meta.Command == "" {
			meta.Command = entry.Name()
		}
		managed = append(managed, meta)
	}

	sort.SliceStable(managed, func(i, j int) bool {
		return managed[i].Command < managed[j].Command
	})
	return managed, nil
}

func readShimMetadata(path string) (managedShimMetadata, bool, error) {
	var meta managedShimMetadata
	fd, err := os.Open(path)
	if err != nil {
		return meta, false, err
	}
	defer fd.Close()

	scanner := bufio.NewScanner(fd)
	isManaged := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == shimMarker {
			isManaged = true
			continue
		}
		if !strings.HasPrefix(line, shimMetaPrefix) {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, shimMetaPrefix))
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		switch key {
		case "COMMAND":
			meta.Command = val
		case "ADAPTER":
			meta.Adapter = val
		case "TARGET":
			meta.Target = val
		case "MODE":
			meta.Mode = val
		case "POLICY_URL":
			meta.PolicyURL = val
		case "EVENT_SINK":
			meta.EventSink = val
		}
	}
	if err := scanner.Err(); err != nil {
		return managedShimMetadata{}, false, err
	}
	return meta, isManaged, nil
}

func buildShimScript(shimBinary, command, adapter, target, mode, policyURL, eventSink string) string {
	lines := []string{
		"#!/bin/sh",
		"set -eu",
		shimMarker,
		"# DATAFOG_SHIM_COMMAND=" + command,
		"# DATAFOG_SHIM_ADAPTER=" + adapter,
		"# DATAFOG_SHIM_TARGET=" + target,
		"# DATAFOG_SHIM_MODE=" + mode,
		"# DATAFOG_SHIM_POLICY_URL=" + policyURL,
		"# DATAFOG_SHIM_EVENT_SINK=" + eventSink,
		"",
		"SHIM_BINARY=" + shQuote(shimBinary),
		"SHIM_MODE=" + shQuote(mode),
		"SHIM_POLICY_URL=" + shQuote(policyURL),
		"SHIM_EVENT_SINK=" + shQuote(eventSink),
		"if [ -n \"${DATAFOG_SHIM_MODE:-}\" ]; then",
		"  SHIM_MODE=\"$DATAFOG_SHIM_MODE\"",
		"fi",
		"if [ -n \"${DATAFOG_SHIM_POLICY_URL:-}\" ]; then",
		"  SHIM_POLICY_URL=\"$DATAFOG_SHIM_POLICY_URL\"",
		"fi",
		"if [ -n \"${DATAFOG_SHIM_EVENT_SINK:-}\" ]; then",
		"  SHIM_EVENT_SINK=\"$DATAFOG_SHIM_EVENT_SINK\"",
		"fi",
		"",
		`exec "$SHIM_BINARY" run \`,
		`  --adapter "` + shellEscape(adapter) + `" \`,
		`  --target "` + shellEscape(target) + `" \`,
		`  --mode "$SHIM_MODE" \`,
		`  --policy-url "$SHIM_POLICY_URL" \`,
		`  --event-sink "$SHIM_EVENT_SINK" \`,
		`  --api-token "${DATAFOG_SHIM_API_TOKEN:-}" \`,
		`  -- \`,
		`  "$@"`,
		"",
	}
	return strings.Join(lines, "\n")
}

func shellEscape(value string) string {
	value = strings.TrimSpace(value)
	return strings.ReplaceAll(value, `"`, `\"`)
}

func shQuote(value string) string {
	value = strings.ReplaceAll(value, `'`, `'\''`)
	return "'" + value + "'"
}

func shimScriptPath(dir, command string) string {
	name := command
	if runtime.GOOS == "windows" && !strings.HasSuffix(strings.ToLower(name), ".cmd") {
		name += ".cmd"
	}
	return filepath.Join(dir, name)
}

func usage() string {
	text := strings.TrimSpace(`
usage:
  datafog-shim --policy-url=http://localhost:8080 shell <command> [args...]
  datafog-shim --policy-url=http://localhost:8080 run --adapter <name> --target <path> [args...]
  datafog-shim --policy-url=http://localhost:8080 read-file <path>
  datafog-shim --policy-url=http://localhost:8080 write-file <path> <text>
  datafog-shim hooks install [--adapter <name>] [--target <path>] <command>
  datafog-shim hooks list
  datafog-shim hooks uninstall <command>
`)
	return text
}
