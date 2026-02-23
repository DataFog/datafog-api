package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datafog/datafog-api/internal/models"
	"github.com/datafog/datafog-api/internal/receipts"
)

func testPolicy() models.Policy {
	return models.Policy{
		PolicyID:      "test",
		PolicyVersion: "v1",
		Rules: []models.Rule{
			{ID: "allow-read", Effect: models.DecisionAllow, Match: models.MatchCriteria{ActionTypes: []string{"file.read"}}, Priority: 10},
			{ID: "transform-write", Effect: models.DecisionTransform, Match: models.MatchCriteria{ActionTypes: []string{"file.write"}}, EntityRequirements: []string{"email"},
				EntityTransforms: []models.TransformStep{{EntityType: "email", Mode: models.TransformModeMask}}},
			{ID: "deny-shell", Effect: models.DecisionDeny, Match: models.MatchCriteria{ActionTypes: []string{"shell.exec"}}, EntityRequirements: []string{"api_key"}},
		},
	}
}

func makeServer(t *testing.T) *http.Server {
	t.Helper()
	store, err := receipts.NewReceiptStore(t.TempDir() + "/receipts.jsonl")
	if err != nil {
		t.Fatalf("new store: %v", err)
	}
	h := New(testPolicy(), store, nil)
	return &http.Server{Handler: h.Handler()}
}

func TestHealthEndpoint(t *testing.T) {
	server := makeServer(t)
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
}

func TestScanEndpoint(t *testing.T) {
	server := makeServer(t)
	body := bytes.NewBufferString(`{"text":"email jane@example.com"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/scan", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var scanned models.ScanResponse
	if err := json.NewDecoder(resp.Body).Decode(&scanned); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(scanned.Findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(scanned.Findings))
	}
}

func TestDecideAndReceiptFlow(t *testing.T) {
	server := makeServer(t)
	body := bytes.NewBufferString(`{"action":{"type":"file.write","resource":"notes.txt"},"text":"contact jane@example.com","request_id":"r1"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/decide", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var decided models.DecideResponse
	if err := json.NewDecoder(resp.Body).Decode(&decided); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decided.Decision != models.DecisionTransform {
		t.Fatalf("expected transform, got %q", decided.Decision)
	}
	if decided.ReceiptID == "" {
		t.Fatalf("expected receipt id")
	}

	req2 := httptest.NewRequest(http.MethodGet, "/v1/receipts/"+decided.ReceiptID, nil)
	resp2 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusOK {
		t.Fatalf("expected 200 receipt, got %d", resp2.Code)
	}
	var saved models.Receipt
	if err := json.NewDecoder(resp2.Body).Decode(&saved); err != nil {
		t.Fatalf("decode receipt failed: %v", err)
	}
	if saved.ReceiptID != decided.ReceiptID {
		t.Fatalf("receipt id mismatch")
	}
}

func TestDenyDecision(t *testing.T) {
	server := makeServer(t)
	body := bytes.NewBufferString(`{"action":{"type":"shell.exec","resource":"curl"},"text":"api_key=ABCD1234EFGH5678"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/decide", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}
	var decided models.DecideResponse
	if err := json.NewDecoder(resp.Body).Decode(&decided); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decided.Decision != models.DecisionDeny {
		t.Fatalf("expected deny, got %q", decided.Decision)
	}
}
