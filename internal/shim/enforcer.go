package shim

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"time"

	"github.com/datafog/datafog-api/internal/models"
	"github.com/datafog/datafog-api/internal/scan"
	"github.com/datafog/datafog-api/internal/transform"
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

type EnforcementMode string

const (
	ModeEnforced EnforcementMode = "enforced"
	ModeObserve  EnforcementMode = "observe"
)

type GateOption func(*Gate)

func WithMode(mode EnforcementMode) GateOption {
	return func(g *Gate) {
		g.Mode = mode
	}
}

func WithEventSink(sink DecisionEventSink) GateOption {
	return func(g *Gate) {
		if sink == nil {
			g.EventSink = noopEventSink{}
			return
		}
		g.EventSink = sink
	}
}

type Gate struct {
	Client    DecisionClient
	Runner    CommandRunner
	Reader    FileReader
	Writer    FileWriter
	Mode      EnforcementMode
	EventSink DecisionEventSink
}

func NewGate(client DecisionClient, opts ...GateOption) *Gate {
	g := &Gate{
		Client:    client,
		Runner:    &osCommandRunner{},
		Reader:    &osFileReader{},
		Writer:    &osFileWriter{},
		Mode:      ModeEnforced,
		EventSink: noopEventSink{},
	}
	for _, opt := range opts {
		if opt != nil {
			opt(g)
		}
	}
	g.normalizeMode()
	return g
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

func (r *Gate) normalizeMode() {
	switch r.Mode {
	case "", ModeEnforced, ModeObserve:
		return
	default:
		r.Mode = ModeEnforced
	}
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

func (r *Gate) shouldAllow(decision models.Decision) bool {
	if r.Mode == "" {
		r.normalizeMode()
	}
	if r.permitDecision(decision) {
		return true
	}
	return r.Mode == ModeObserve
}

func (r *Gate) executeRequest(ctx context.Context, req models.DecideRequest, run func(context.Context) ([]byte, error)) (models.DecideResponse, []byte, error) {
	result, err := r.Check(ctx, req)
	if err != nil {
		if r.Mode == ModeEnforced {
			r.recordDecisionEvent(req, result, false, err)
			return result, nil, err
		}
		fallback := models.DecideResponse{
			Decision:  models.DecisionAllow,
			Reason:    err.Error(),
			RequestID: req.RequestID,
			TraceID:   req.TraceID,
		}
		r.recordDecisionEvent(req, fallback, true, err)
		output, runErr := run(ctx)
		return fallback, output, runErr
	}
	if !r.shouldAllow(result.Decision) {
		r.recordDecisionEvent(req, result, false, nil)
		return result, nil, &PolicyDecisionError{Response: result}
	}
	r.recordDecisionEvent(req, result, true, nil)
	output, runErr := run(ctx)
	return result, output, runErr
}

func (r *Gate) readRequest(action models.ActionMeta, text string, findings []models.ScanFinding) models.DecideRequest {
	return models.DecideRequest{
		Action:    action,
		Text:      text,
		Findings:  findings,
		RequestID: "",
		TraceID:   "",
	}
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
		Args:      append([]string(nil), args...),
		Sensitive: sensitive,
	}
	return r.executeRequest(ctx, r.readRequest(action, text, findings), func(ctx context.Context) ([]byte, error) {
		return r.Runner.Run(ctx, command, args...)
	})
}

func (r *Gate) ReadFile(ctx context.Context, path string, text string, findings []models.ScanFinding, sensitive bool) (models.DecideResponse, []byte, error) {
	if r.Reader == nil {
		return models.DecideResponse{}, nil, fmt.Errorf("file reader is not configured")
	}
	action := models.ActionMeta{
		Type:      "file.read",
		Tool:      "fs",
		Resource:  path,
		Sensitive: sensitive,
	}
	result, output, err := r.executeRequest(ctx, r.readRequest(action, text, findings), func(ctx context.Context) ([]byte, error) {
		return r.Reader.ReadFile(path)
	})
	if err != nil {
		return result, output, err
	}

	// Apply redaction to read output when decision is allow_with_redaction
	if result.Decision == models.DecisionAllowWithRedaction && len(result.TransformPlan) > 0 && output != nil {
		output = r.applyRedaction(output, result.TransformPlan, nil)
	}

	return result, output, nil
}

func (r *Gate) WriteFile(ctx context.Context, path string, data []byte, perm fs.FileMode, text string, findings []models.ScanFinding, sensitive bool) (models.DecideResponse, error) {
	if r.Writer == nil {
		return models.DecideResponse{}, fmt.Errorf("file writer is not configured")
	}
	action := models.ActionMeta{
		Type:      "file.write",
		Tool:      "fs",
		Resource:  path,
		Sensitive: sensitive,
	}
	req := r.readRequest(action, text, findings)
	result, err := r.Check(ctx, req)
	if err != nil {
		if r.Mode == ModeEnforced {
			r.recordDecisionEvent(req, result, false, err)
			return result, err
		}
		fallback := models.DecideResponse{
			Decision:  models.DecisionAllow,
			Reason:    err.Error(),
			RequestID: req.RequestID,
			TraceID:   req.TraceID,
		}
		r.recordDecisionEvent(req, fallback, true, err)
		return fallback, r.Writer.WriteFile(path, data, perm)
	}
	if !r.shouldAllow(result.Decision) {
		r.recordDecisionEvent(req, result, false, nil)
		return result, &PolicyDecisionError{Response: result}
	}

	// Apply transform plan on allow_with_redaction
	writeData := data
	if result.Decision == models.DecisionAllowWithRedaction && len(result.TransformPlan) > 0 {
		writeData = r.applyRedaction(data, result.TransformPlan, findings)
	}

	r.recordDecisionEvent(req, result, true, nil)
	return result, r.Writer.WriteFile(path, writeData, perm)
}

func (r *Gate) ExecuteCommand(ctx context.Context, adapterName string, target string, args []string, text string, findings []models.ScanFinding, sensitive bool) (models.DecideResponse, []byte, error) {
	if r.Runner == nil {
		return models.DecideResponse{}, nil, fmt.Errorf("command runner is not configured")
	}
	if adapterName == "" {
		return models.DecideResponse{}, nil, fmt.Errorf("adapter name is required")
	}
	if target == "" {
		return models.DecideResponse{}, nil, fmt.Errorf("target binary is required")
	}

	action := models.ActionMeta{
		Type:      "command.exec",
		Tool:      adapterName,
		Resource:  target,
		Sensitive: sensitive,
	}
	if len(args) > 0 {
		action.Command = args[0]
		action.Args = append([]string(nil), args...)
	}
	return r.executeRequest(ctx, r.readRequest(action, text, findings), func(ctx context.Context) ([]byte, error) {
		return r.Runner.Run(ctx, target, args...)
	})
}

func (r *Gate) recordDecisionEvent(req models.DecideRequest, decision models.DecideResponse, allowed bool, checkErr error) {
	if r.EventSink == nil {
		r.EventSink = noopEventSink{}
	}
	r.EventSink.Record(DecisionEvent{
		Timestamp:  time.Now().UTC(),
		Mode:       string(r.Mode),
		ActionType: req.Action.Type,
		Tool:       req.Action.Tool,
		Resource:   req.Action.Resource,
		Command:    req.Action.Command,
		Args:       append([]string(nil), req.Action.Args...),
		Sensitive:  req.Action.Sensitive,
		Decision:   string(decision.Decision),
		Allowed:    allowed,
		ReceiptID:  decision.ReceiptID,
		Matched:    append([]string(nil), decision.MatchedRules...),
		Reason:     decision.Reason,
		CheckError: errorString(checkErr),
		RequestID:  req.RequestID,
		TraceID:    req.TraceID,
	})
}

// applyRedaction scans the content for PII and applies the transform plan.
// If findings are provided, they are used directly; otherwise the content is scanned.
func (r *Gate) applyRedaction(data []byte, plan []models.TransformStep, findings []models.ScanFinding) []byte {
	text := string(data)
	if len(findings) == 0 {
		findings = scan.ScanText(text, nil)
	}
	if len(findings) == 0 {
		return data
	}
	output, _ := transform.ApplyTransforms(text, findings, plan)
	return []byte(output)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
