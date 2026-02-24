package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/datafog/datafog-api/internal/models"
	"github.com/datafog/datafog-api/internal/scan"
	"github.com/datafog/datafog-api/internal/shim"
)

// DemoHandler exposes endpoints that execute real commands and file
// operations through the shim gate. Must be explicitly enabled.
type DemoHandler struct {
	gate       *shim.Gate
	sandboxDir string
	server     *Server
	demoHTML   []byte
}

type demoExecRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args"`
	Stdin   string   `json:"stdin,omitempty"`
}

type demoExecResponse struct {
	Decision models.DecideResponse `json:"decision"`
	Stdout   string                `json:"stdout"`
	Stderr   string                `json:"stderr"`
	Error    string                `json:"error,omitempty"`
	Blocked  bool                  `json:"blocked"`
	TimingMs int64                 `json:"timing_ms"`
	Findings []models.ScanFinding  `json:"findings,omitempty"`
}

type demoWriteRequest struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

type demoWriteResponse struct {
	Decision models.DecideResponse `json:"decision"`
	Written  bool                  `json:"written"`
	Path     string                `json:"path"`
	Content  string                `json:"content"`
	Error    string                `json:"error,omitempty"`
	Blocked  bool                  `json:"blocked"`
	TimingMs int64                 `json:"timing_ms"`
	Findings []models.ScanFinding  `json:"findings,omitempty"`
}

type demoReadRequest struct {
	Filename string `json:"filename"`
}

type demoReadResponse struct {
	Decision models.DecideResponse `json:"decision"`
	Content  string                `json:"content"`
	Error    string                `json:"error,omitempty"`
	Blocked  bool                  `json:"blocked"`
	TimingMs int64                 `json:"timing_ms"`
	Findings []models.ScanFinding  `json:"findings,omitempty"`
}

// NewDemoHandler creates a demo handler backed by the given gate.
// It creates a sandbox directory for file operations.
func NewDemoHandler(gate *shim.Gate, srv *Server, demoHTMLPath string) (*DemoHandler, error) {
	sandboxDir, err := os.MkdirTemp("", "datafog-demo-*")
	if err != nil {
		return nil, fmt.Errorf("create demo sandbox: %w", err)
	}
	var html []byte
	if demoHTMLPath != "" {
		html, err = os.ReadFile(demoHTMLPath)
		if err != nil {
			return nil, fmt.Errorf("read demo HTML: %w", err)
		}
	}
	return &DemoHandler{
		gate:       gate,
		sandboxDir: sandboxDir,
		server:     srv,
		demoHTML:   html,
	}, nil
}

// Cleanup removes the sandbox directory.
func (d *DemoHandler) Cleanup() {
	if d.sandboxDir != "" {
		os.RemoveAll(d.sandboxDir)
	}
}

// Register adds the demo endpoints to the given mux.
func (d *DemoHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/demo", d.handleDemoPage)
	mux.HandleFunc("/demo/exec", d.handleExec)
	mux.HandleFunc("/demo/write-file", d.handleWriteFile)
	mux.HandleFunc("/demo/read-file", d.handleReadFile)
	mux.HandleFunc("/demo/seed", d.handleSeed)
	mux.HandleFunc("/demo/sandbox", d.handleSandboxInfo)
}

func (d *DemoHandler) handleDemoPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Write(d.demoHTML)
}

func (d *DemoHandler) handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		d.server.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be POST"})
		return
	}

	var req demoExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		d.server.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: err.Error()})
		return
	}
	if req.Command == "" {
		d.server.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "command is required"})
		return
	}

	// Scan the stdin/context for PII
	textToScan := req.Stdin
	if textToScan == "" {
		textToScan = req.Command + " " + strings.Join(req.Args, " ")
	}
	findings := scan.ScanText(textToScan, nil)

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	decision, output, err := d.gate.ExecuteShell(ctx, req.Command, req.Args, textToScan, findings, len(findings) > 0)
	elapsed := time.Since(start).Milliseconds()

	resp := demoExecResponse{
		Decision: decision,
		TimingMs: elapsed,
		Findings: findings,
	}

	if err != nil {
		resp.Blocked = true
		resp.Error = err.Error()
	} else {
		resp.Stdout = string(output)
	}

	d.server.respond(w, http.StatusOK, resp)
}

func (d *DemoHandler) handleWriteFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		d.server.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be POST"})
		return
	}

	var req demoWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		d.server.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: err.Error()})
		return
	}
	if req.Filename == "" {
		d.server.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "filename is required"})
		return
	}

	// Sanitize filename to prevent directory traversal
	cleanName := filepath.Base(req.Filename)
	fullPath := filepath.Join(d.sandboxDir, cleanName)

	// Scan content for PII
	findings := scan.ScanText(req.Content, nil)

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	decision, err := d.gate.WriteFile(ctx, fullPath, []byte(req.Content), 0o600, req.Content, findings, len(findings) > 0)
	elapsed := time.Since(start).Milliseconds()

	resp := demoWriteResponse{
		Decision: decision,
		Path:     cleanName,
		TimingMs: elapsed,
		Findings: findings,
	}

	if err != nil {
		resp.Blocked = true
		resp.Error = err.Error()
	} else {
		resp.Written = true
		// Read back what was actually written (may be redacted)
		if data, readErr := os.ReadFile(fullPath); readErr == nil {
			resp.Content = string(data)
		}
	}

	d.server.respond(w, http.StatusOK, resp)
}

func (d *DemoHandler) handleReadFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		d.server.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be POST"})
		return
	}

	var req demoReadRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		d.server.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: err.Error()})
		return
	}
	if req.Filename == "" {
		d.server.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "filename is required"})
		return
	}

	cleanName := filepath.Base(req.Filename)
	fullPath := filepath.Join(d.sandboxDir, cleanName)

	start := time.Now()
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	decision, output, err := d.gate.ReadFile(ctx, fullPath, "", nil, false)
	elapsed := time.Since(start).Milliseconds()

	resp := demoReadResponse{
		Decision: decision,
		TimingMs: elapsed,
	}

	if err != nil {
		resp.Blocked = true
		resp.Error = err.Error()
	} else {
		resp.Content = string(output)
		// Scan the output to report what was found
		resp.Findings = scan.ScanText(string(output), nil)
	}

	d.server.respond(w, http.StatusOK, resp)
}

// handleSeed writes a file directly to the sandbox, bypassing the shim gate.
// This lets demo scenarios place raw PII on disk so that a subsequent
// gated read can demonstrate redaction on the way out.
func (d *DemoHandler) handleSeed(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		d.server.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be POST"})
		return
	}

	var req demoWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		d.server.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: err.Error()})
		return
	}
	if req.Filename == "" || req.Content == "" {
		d.server.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "filename and content are required"})
		return
	}

	cleanName := filepath.Base(req.Filename)
	fullPath := filepath.Join(d.sandboxDir, cleanName)

	if err := os.WriteFile(fullPath, []byte(req.Content), 0o600); err != nil {
		d.server.respondError(w, http.StatusInternalServerError, models.APIError{Code: "seed_error", Message: err.Error()})
		return
	}

	d.server.respond(w, http.StatusOK, map[string]interface{}{
		"seeded":   true,
		"filename": cleanName,
	})
}

func (d *DemoHandler) handleSandboxInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		d.server.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be GET"})
		return
	}

	entries, _ := os.ReadDir(d.sandboxDir)
	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			files = append(files, entry.Name())
		}
	}

	d.server.respond(w, http.StatusOK, map[string]interface{}{
		"sandbox_dir": d.sandboxDir,
		"files":       files,
	})
}
