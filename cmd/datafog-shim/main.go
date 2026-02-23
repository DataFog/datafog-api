package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/datafog/datafog-api/internal/shim"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(argv []string) error {
	flags := flag.NewFlagSet("datafog-shim", flag.ContinueOnError)
	policyURL := flags.String("policy-url", "http://localhost:8080", "base URL for datafog API (for example http://localhost:8080)")
	apiToken := flags.String("api-token", "", "API token for datafog authorize endpoint")
	sensitive := flags.Bool("sensitive", false, "mark this action as sensitive")
	if err := flags.Parse(argv); err != nil {
		return err
	}

	args := flags.Args()
	if len(args) == 0 {
		return fmt.Errorf("missing command: shell|read-file|write-file\n\n%s", usage())
	}

	client := shim.NewHTTPDecisionClient(*policyURL, *apiToken)
	gate := shim.NewGate(client)

	cmd := args[0]
	switch cmd {
	case "shell":
		return runShell(context.Background(), gate, args[1:], *sensitive)
	case "read-file":
		return runReadFile(context.Background(), gate, args[1:], *sensitive)
	case "write-file":
		return runWriteFile(context.Background(), gate, args[1:], *sensitive)
	default:
		return fmt.Errorf("unknown command %q\n\n%s", cmd, usage())
	}
}

func runShell(ctx context.Context, gate *shim.Gate, args []string, sensitive bool) error {
	if len(args) < 1 {
		return fmt.Errorf("shell command is required")
	}
	command := args[0]
	shellArgs := args[1:]
	decision, output, err := gate.ExecuteShell(ctx, command, shellArgs, "", nil, sensitive)
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

func runReadFile(ctx context.Context, gate *shim.Gate, args []string, sensitive bool) error {
	if len(args) < 1 {
		return fmt.Errorf("read-file path is required")
	}
	path := args[0]
	decision, data, err := gate.ReadFile(ctx, path, "", nil, sensitive)
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

func runWriteFile(ctx context.Context, gate *shim.Gate, args []string, sensitive bool) error {
	if len(args) < 2 {
		return fmt.Errorf("write-file requires <path> <text>")
	}
	path := args[0]
	content := strings.Join(args[1:], " ")
	decision, err := gate.WriteFile(ctx, path, []byte(content), 0o600, "", nil, sensitive)
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %d bytes to %s receipt=%s decision=%s\n", len(content), path, decision.ReceiptID, decision.Decision)
	return nil
}

func usage() string {
	text := strings.TrimSpace(`
usage:
  datafog-shim --policy-url=http://localhost:8080 shell <command> [args...]
  datafog-shim --policy-url=http://localhost:8080 read-file <path>
  datafog-shim --policy-url=http://localhost:8080 write-file <path> <text>
`)
	return text
}
