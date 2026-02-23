package shim

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"

	"github.com/datafog/datafog-api/internal/models"
)

type CommandRunner interface {
	Run(ctx context.Context, command string, args ...string) ([]byte, error)
}

type FileReader interface {
	ReadFile(path string) ([]byte, error)
}

type FileWriter interface {
	WriteFile(path string, data []byte, perm fs.FileMode) error
}

type Gate struct {
	Client DecisionClient
	Runner CommandRunner
	Reader FileReader
	Writer FileWriter
}

func NewGate(client DecisionClient) *Gate {
	return &Gate{
		Client: client,
		Runner: &osCommandRunner{},
		Reader: &osFileReader{},
		Writer: &osFileWriter{},
	}
}

type osCommandRunner struct{}

func (r *osCommandRunner) Run(ctx context.Context, command string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, command, args...) // #nosec G204 -- command execution is an explicit policy-gated feature.
	return cmd.CombinedOutput()
}

type osFileReader struct{}

func (r *osFileReader) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

type osFileWriter struct{}

func (r *osFileWriter) WriteFile(path string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(path, data, perm)
}

type PolicyDecisionError struct {
	Response models.DecideResponse
}

func (e *PolicyDecisionError) Error() string {
	return fmt.Sprintf("policy denied action decision=%s rules=%v reason=%v", e.Response.Decision, e.Response.MatchedRules, e.Response.Reason)
}

func (r *Gate) Check(ctx context.Context, req models.DecideRequest) (models.DecideResponse, error) {
	if r.Client == nil {
		return models.DecideResponse{}, fmt.Errorf("policy decision client is not configured")
	}
	return r.Client.Decide(ctx, req)
}

func (r *Gate) permitDecision(decision models.Decision) bool {
	switch decision {
	case models.DecisionAllow, models.DecisionAllowWithRedaction:
		return true
	default:
		return false
	}
}

func (r *Gate) enforceBeforeAction(ctx context.Context, req models.DecideRequest) (models.DecideResponse, error) {
	result, err := r.Check(ctx, req)
	if err != nil {
		return models.DecideResponse{}, err
	}
	if !r.permitDecision(result.Decision) {
		return result, &PolicyDecisionError{Response: result}
	}
	return result, nil
}

func (r *Gate) ExecuteShell(ctx context.Context, command string, args []string, text string, findings []models.ScanFinding, sensitive bool) (models.DecideResponse, []byte, error) {
	if r.Runner == nil {
		return models.DecideResponse{}, nil, fmt.Errorf("command runner is not configured")
	}
	action := models.ActionMeta{
		Type:      "shell.exec",
		Tool:      "shell",
		Resource:  command,
		Command:   command,
		Args:      args,
		Sensitive: sensitive,
	}
	result, err := r.enforceBeforeAction(ctx, models.DecideRequest{
		Action:   action,
		Text:     text,
		Findings: findings,
	})
	if err != nil {
		return result, nil, err
	}
	output, err := r.Runner.Run(ctx, command, args...)
	return result, output, err
}

func (r *Gate) ReadFile(ctx context.Context, path string, text string, findings []models.ScanFinding, sensitive bool) (models.DecideResponse, []byte, error) {
	if r.Reader == nil {
		return models.DecideResponse{}, nil, fmt.Errorf("file reader is not configured")
	}
	result, err := r.enforceBeforeAction(ctx, models.DecideRequest{
		Action: models.ActionMeta{
			Type:      "file.read",
			Tool:      "fs",
			Resource:  path,
			Sensitive: sensitive,
		},
		Text:     text,
		Findings: findings,
	})
	if err != nil {
		return result, nil, err
	}
	data, err := r.Reader.ReadFile(path)
	return result, data, err
}

func (r *Gate) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode, text string, findings []models.ScanFinding, sensitive bool) (models.DecideResponse, error) {
	if r.Writer == nil {
		return models.DecideResponse{}, fmt.Errorf("file writer is not configured")
	}
	result, err := r.enforceBeforeAction(ctx, models.DecideRequest{
		Action: models.ActionMeta{
			Type:      "file.write",
			Tool:      "fs",
			Resource:  path,
			Sensitive: sensitive,
		},
		Text:     text,
		Findings: findings,
	})
	if err != nil {
		return result, err
	}
	if err := r.Writer.WriteFile(path, data, perm); err != nil {
		return result, err
	}
	return result, nil
}
