package shim

import (
	"context"
	"errors"
	"io/fs"
	"testing"

	"github.com/datafog/datafog-api/internal/models"
)

type fakeDecisionClient struct {
	response models.DecideResponse
	err      error
	calls    int
	lastReq  models.DecideRequest
}

func (c *fakeDecisionClient) Decide(ctx context.Context, req models.DecideRequest) (models.DecideResponse, error) {
	c.calls++
	c.lastReq = req
	if c.err != nil {
		return models.DecideResponse{}, c.err
	}
	return c.response, nil
}

type fakeCommandRunner struct {
	called bool
	cmd    string
	args   []string
	out    []byte
	err    error
}

func (r *fakeCommandRunner) Run(ctx context.Context, command string, args ...string) ([]byte, error) {
	r.called = true
	r.cmd = command
	r.args = append([]string{}, args...)
	return r.out, r.err
}

type fakeFileReader struct {
	called bool
	path   string
	data   []byte
	err    error
}

func (r *fakeFileReader) ReadFile(path string) ([]byte, error) {
	r.called = true
	r.path = path
	return r.data, r.err
}

type fakeFileWriter struct {
	called bool
	path   string
	data   []byte
	perm   fs.FileMode
	err    error
}

func (r *fakeFileWriter) WriteFile(path string, data []byte, perm fs.FileMode) error {
	r.called = true
	r.path = path
	r.data = append([]byte{}, data...)
	r.perm = perm
	return r.err
}

type fakeEventRecorder struct {
	events []DecisionEvent
}

func (r *fakeEventRecorder) Record(event DecisionEvent) {
	r.events = append(r.events, event)
}

func TestShellExecutionAllowed(t *testing.T) {
	decision := models.DecideResponse{
		Decision:     models.DecisionAllow,
		ReceiptID:    "r1",
		MatchedRules: []string{"allow-shell"},
	}
	decider := &fakeDecisionClient{response: decision}
	runner := &fakeCommandRunner{out: []byte("ok\n")}
	interceptor := &Gate{
		Client: decider,
		Runner: runner,
	}

	res, out, err := interceptor.ExecuteShell(context.Background(), "ls", []string{"-la"}, "file ls -la", []models.ScanFinding{}, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if string(out) != "ok\n" {
		t.Fatalf("expected command output, got %q", string(out))
	}
	if !runner.called {
		t.Fatalf("expected command runner to be called")
	}
	if decider.calls != 1 {
		t.Fatalf("expected one policy call, got %d", decider.calls)
	}
	if decider.lastReq.Action.Type != "shell.exec" {
		t.Fatalf("expected shell action type, got %q", decider.lastReq.Action.Type)
	}
	if decider.lastReq.Action.Command != "ls" {
		t.Fatalf("expected command ls, got %q", decider.lastReq.Action.Command)
	}
	if decider.lastReq.Action.Args[0] != "-la" {
		t.Fatalf("expected first arg -la, got %q", decider.lastReq.Action.Args[0])
	}
	if runner.cmd != "ls" || runner.args[0] != "-la" {
		t.Fatalf("expected shell invocation, got %q %v", runner.cmd, runner.args)
	}
	if res.ReceiptID != "r1" {
		t.Fatalf("expected receipt id, got %q", res.ReceiptID)
	}
	if !decider.lastReq.Action.Sensitive {
		t.Fatalf("expected sensitive field to be preserved")
	}
}

func TestShellExecutionDenied(t *testing.T) {
	decider := &fakeDecisionClient{
		response: models.DecideResponse{
			Decision:     models.DecisionDeny,
			Reason:       "blocked command",
			MatchedRules: []string{"deny-shell"},
		},
	}
	runner := &fakeCommandRunner{out: []byte("ok")}
	interceptor := &Gate{
		Client: decider,
		Runner: runner,
	}

	_, _, err := interceptor.ExecuteShell(context.Background(), "rm", []string{"-rf", "/tmp"}, "", nil, false)
	if err == nil {
		t.Fatalf("expected denied action error")
	}
	var denied *PolicyDecisionError
	if !errors.As(err, &denied) {
		t.Fatalf("expected PolicyDecisionError, got %T", err)
	}
	if runner.called {
		t.Fatalf("expected command runner to be skipped on deny")
	}
	if len(denied.Response.MatchedRules) != 1 || denied.Response.MatchedRules[0] != "deny-shell" {
		t.Fatalf("expected denied rule reason, got %+v", denied.Response.MatchedRules)
	}
}

func TestShellExecutionAllowsInObserveMode(t *testing.T) {
	decider := &fakeDecisionClient{
		response: models.DecideResponse{
			Decision:     models.DecisionDeny,
			ReceiptID:    "r2",
			MatchedRules: []string{"deny-shell"},
		},
	}
	runner := &fakeCommandRunner{out: []byte("ok\n")}
	recorder := &fakeEventRecorder{}
	interceptor := NewGate(decider, WithMode(ModeObserve), WithEventSink(recorder))
	interceptor.Runner = runner

	res, out, err := interceptor.ExecuteShell(context.Background(), "rm", []string{"-rf", "/tmp"}, "", []models.ScanFinding{}, false)
	if err != nil {
		t.Fatalf("expected no error in observe mode, got %v", err)
	}
	if string(out) != "ok\n" {
		t.Fatalf("expected command output, got %q", string(out))
	}
	if res.Decision != models.DecisionDeny {
		t.Fatalf("expected deny decision for observability, got %q", res.Decision)
	}
	if !runner.called {
		t.Fatalf("expected runner to execute in observe mode")
	}
	if len(recorder.events) != 1 {
		t.Fatalf("expected one event, got %d", len(recorder.events))
	}
	if recorder.events[0].Mode != string(ModeObserve) {
		t.Fatalf("expected observe event mode, got %q", recorder.events[0].Mode)
	}
}

func TestShellExecutionPolicyErrorPassesInObserveMode(t *testing.T) {
	decider := &fakeDecisionClient{
		err: errors.New("policy unavailable"),
	}
	runner := &fakeCommandRunner{out: []byte("ok\n")}
	recorder := &fakeEventRecorder{}
	interceptor := NewGate(decider, WithMode(ModeObserve), WithEventSink(recorder))
	interceptor.Runner = runner

	_, out, err := interceptor.ExecuteShell(context.Background(), "ls", nil, "", nil, false)
	if err != nil {
		t.Fatalf("expected no error when API is unreachable in observe mode, got %v", err)
	}
	if string(out) != "ok\n" {
		t.Fatalf("expected command output, got %q", string(out))
	}
	if len(recorder.events) != 1 || recorder.events[0].CheckError == "" {
		t.Fatalf("expected policy error in event, got %#v", recorder.events)
	}
}

func TestCommandAdapterExecution(t *testing.T) {
	decider := &fakeDecisionClient{
		response: models.DecideResponse{
			Decision: models.DecisionAllow,
		},
	}
	runner := &fakeCommandRunner{out: []byte("run\n")}
	interceptor := &Gate{
		Client: decider,
		Runner: runner,
	}

	res, out, err := interceptor.ExecuteCommand(context.Background(), "git", "/usr/bin/git", []string{"status"}, "", nil, false)
	if err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
	if string(out) != "run\n" {
		t.Fatalf("expected command output, got %q", string(out))
	}
	if !runner.called {
		t.Fatalf("expected command runner")
	}
	if runner.cmd != "/usr/bin/git" {
		t.Fatalf("expected target binary, got %q", runner.cmd)
	}
	if decider.lastReq.Action.Type != "command.exec" {
		t.Fatalf("expected command.exec action, got %q", decider.lastReq.Action.Type)
	}
	if decider.lastReq.Action.Tool != "git" {
		t.Fatalf("expected tool git, got %q", decider.lastReq.Action.Tool)
	}
	if decider.lastReq.Action.Command != "status" {
		t.Fatalf("expected command to be first arg, got %q", decider.lastReq.Action.Command)
	}
	if res.ReceiptID != "" {
		t.Fatalf("did not expect receipt id in mocked response")
	}
}

func TestReadFileAllowed(t *testing.T) {
	decider := &fakeDecisionClient{
		response: models.DecideResponse{
			Decision: models.DecisionAllow,
		},
	}
	reader := &fakeFileReader{data: []byte("payload")}
	interceptor := &Gate{
		Client: decider,
		Reader: reader,
	}

	_, data, err := interceptor.ReadFile(context.Background(), "/tmp/a.txt", "", []models.ScanFinding{{EntityType: "email", Value: "x@y.z", Start: 0, End: 5, Confidence: 0.9}}, false)
	if err != nil {
		t.Fatalf("expected allow, got %v", err)
	}
	if string(data) != "payload" {
		t.Fatalf("expected payload, got %q", string(data))
	}
	if reader.path != "/tmp/a.txt" {
		t.Fatalf("expected reader path, got %q", reader.path)
	}
	if decider.lastReq.Action.Type != "file.read" {
		t.Fatalf("expected file.read action, got %q", decider.lastReq.Action.Type)
	}
}

func TestWriteFileAllowed(t *testing.T) {
	decider := &fakeDecisionClient{
		response: models.DecideResponse{
			Decision: models.DecisionAllow,
		},
	}
	writer := &fakeFileWriter{}
	interceptor := &Gate{
		Client: decider,
		Writer: writer,
	}

	_, err := interceptor.WriteFile(context.Background(), "/tmp/a.txt", []byte("payload"), 0o600, "note", []models.ScanFinding{}, true)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if writer.path != "/tmp/a.txt" {
		t.Fatalf("expected write path %q, got %q", "/tmp/a.txt", writer.path)
	}
	if string(writer.data) != "payload" {
		t.Fatalf("expected payload write, got %q", string(writer.data))
	}
	if writer.perm != 0o600 {
		t.Fatalf("expected perm 600, got %v", writer.perm)
	}
}
