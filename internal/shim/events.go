package shim

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type DecisionEvent struct {
	Timestamp  time.Time `json:"timestamp"`
	Mode       string    `json:"mode"`
	ActionType string    `json:"action_type"`
	Tool       string    `json:"tool"`
	Resource   string    `json:"resource"`
	Command    string    `json:"command"`
	Args       []string  `json:"args"`
	Sensitive  bool      `json:"sensitive"`
	Decision   string    `json:"decision"`
	Allowed    bool      `json:"allowed"`
	ReceiptID  string    `json:"receipt_id,omitempty"`
	Matched    []string  `json:"matched_rules,omitempty"`
	Reason     string    `json:"reason,omitempty"`
	CheckError string    `json:"check_error,omitempty"`
	RequestID  string    `json:"request_id,omitempty"`
	TraceID    string    `json:"trace_id,omitempty"`
}

type DecisionEventSink interface {
	Record(event DecisionEvent)
}

type noopEventSink struct{}

func (s noopEventSink) Record(_ DecisionEvent) {}

type NDJSONDecisionEventSink struct {
	path string
	mu   sync.Mutex
}

func NewNDJSONDecisionEventSink(path string) *NDJSONDecisionEventSink {
	return &NDJSONDecisionEventSink{path: path}
}

func (s *NDJSONDecisionEventSink) Record(event DecisionEvent) {
	if s == nil || s.path == "" {
		return
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0o750); err != nil {
		return
	}

	payload, err := json.Marshal(event)
	if err != nil {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	file, err := os.OpenFile(s.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer file.Close()

	_, _ = fmt.Fprintln(file, string(payload))
}
