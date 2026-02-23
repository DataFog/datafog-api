package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

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
	var got models.HealthResponse
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got.Status != "ok" {
		t.Fatalf("expected status ok, got %q", got.Status)
	}
	if got.PolicyID != "test" {
		t.Fatalf("expected policy id test, got %q", got.PolicyID)
	}
	if got.PolicyVersion != "v1" {
		t.Fatalf("expected policy version v1, got %q", got.PolicyVersion)
	}
	if _, err := time.Parse(time.RFC3339, got.StartedAt); err != nil {
		t.Fatalf("expected valid RFC3339 started_at, got %q", got.StartedAt)
	}
}

func TestPolicyVersionEndpoint(t *testing.T) {
	server := makeServer(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/policy/version", nil)
	resp := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var got struct {
		PolicyID      string `json:"policy_id"`
		PolicyVersion string `json:"policy_version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got.PolicyID != "test" {
		t.Fatalf("expected policy id test, got %q", got.PolicyID)
	}
	if got.PolicyVersion != "v1" {
		t.Fatalf("expected policy version v1, got %q", got.PolicyVersion)
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
	if scanned.RequestID != "" {
		t.Fatalf("expected empty request id, got %q", scanned.RequestID)
	}
	if scanned.PolicyID != "test" {
		t.Fatalf("expected policy id test, got %q", scanned.PolicyID)
	}
	if scanned.PolicyVersion != "v1" {
		t.Fatalf("expected policy version v1, got %q", scanned.PolicyVersion)
	}
	if len(scanned.Findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(scanned.Findings))
	}
}

func TestTransformEndpoint(t *testing.T) {
	server := makeServer(t)
	body := bytes.NewBufferString(`{"text":"contact jane@example.com","mode":"mask"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/transform", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var transformed models.TransformResponse
	if err := json.NewDecoder(resp.Body).Decode(&transformed); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if transformed.PolicyID != "test" {
		t.Fatalf("expected policy id test, got %q", transformed.PolicyID)
	}
	if transformed.PolicyVersion != "v1" {
		t.Fatalf("expected policy version v1, got %q", transformed.PolicyVersion)
	}
	if transformed.Stats.EntitiesTransformed == 0 {
		t.Fatalf("expected transformed entity count > 0")
	}
	if transformed.Stats.ModesApplied == "" {
		t.Fatalf("expected modes applied")
	}
	if strings.Contains(transformed.Output, "jane@example.com") {
		t.Fatalf("expected redacted output, got %q", transformed.Output)
	}
}

func TestAnonymizeEndpoint(t *testing.T) {
	server := makeServer(t)
	body := bytes.NewBufferString(`{"text":"contact jane@example.com","findings":[{"entity_type":"email","value":"jane@example.com","start":8,"end":23,"confidence":0.99}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/anonymize", body)
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.Code)
	}

	var anonymized models.TransformResponse
	if err := json.NewDecoder(resp.Body).Decode(&anonymized); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if anonymized.PolicyID != "test" {
		t.Fatalf("expected policy id test, got %q", anonymized.PolicyID)
	}
	if anonymized.Stats.EntitiesTransformed != 1 {
		t.Fatalf("expected one transformed entity, got %d", anonymized.Stats.EntitiesTransformed)
	}
	if strings.Contains(anonymized.Output, "jane@example.com") {
		t.Fatalf("expected anonymized output, got %q", anonymized.Output)
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

func TestDecideIdempotentReplay(t *testing.T) {
	server := makeServer(t)
	body1 := bytes.NewBufferString(`{"action":{"type":"file.write","resource":"notes.txt"},"text":"contact jane@example.com","request_id":"r1","idempotency_key":"idem-1"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/v1/decide", body1)
	req1.Header.Set("Content-Type", "application/json")
	resp1 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp1, req1)
	if resp1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.Code)
	}
	var first models.DecideResponse
	if err := json.NewDecoder(resp1.Body).Decode(&first); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	body2 := bytes.NewBufferString(`{"action":{"type":"file.write","resource":"notes.txt"},"text":"contact jane@example.com","request_id":"r2","idempotency_key":"idem-1"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/decide", body2)
	req2.Header.Set("Content-Type", "application/json")
	resp2 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.Code)
	}
	var second models.DecideResponse
	if err := json.NewDecoder(resp2.Body).Decode(&second); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if first.ReceiptID == "" || second.ReceiptID == "" {
		t.Fatalf("expected receipt ids")
	}
	if first.ReceiptID != second.ReceiptID {
		t.Fatalf("expected same receipt for idempotent requests, got %q and %q", first.ReceiptID, second.ReceiptID)
	}
	if first.Decision != models.DecisionTransform || second.Decision != models.DecisionTransform {
		t.Fatalf("expected transform decisions, got %q and %q", first.Decision, second.Decision)
	}
}

func TestDecideIdempotencyConflict(t *testing.T) {
	server := makeServer(t)
	body1 := bytes.NewBufferString(`{"action":{"type":"file.write","resource":"notes.txt"},"text":"contact jane@example.com","idempotency_key":"idem-conflict"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/v1/decide", body1)
	req1.Header.Set("Content-Type", "application/json")
	resp1 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp1, req1)
	if resp1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.Code)
	}

	body2 := bytes.NewBufferString(`{"action":{"type":"file.write","resource":"notes.txt"},"text":"contact different@example.com","idempotency_key":"idem-conflict"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/decide", body2)
	req2.Header.Set("Content-Type", "application/json")
	resp2 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp2, req2)
	assertJSONError(t, resp2, http.StatusConflict, "idempotency_conflict")
}

func TestScanIdempotentReplay(t *testing.T) {
	server := makeServer(t)
	body1 := bytes.NewBufferString(`{"text":"contact jane@example.com","idempotency_key":"scan-idem-1","request_id":"r1"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/v1/scan", body1)
	req1.Header.Set("Content-Type", "application/json")
	resp1 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp1, req1)
	if resp1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.Code)
	}
	var first models.ScanResponse
	if err := json.NewDecoder(resp1.Body).Decode(&first); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	body2 := bytes.NewBufferString(`{"text":"contact jane@example.com","idempotency_key":"scan-idem-1","request_id":"r2"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/scan", body2)
	req2.Header.Set("Content-Type", "application/json")
	resp2 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.Code)
	}
	var second models.ScanResponse
	if err := json.NewDecoder(resp2.Body).Decode(&second); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if len(first.Findings) != len(second.Findings) || first.Findings[0].EntityType != second.Findings[0].EntityType {
		t.Fatalf("expected identical findings")
	}
}

func TestScanIdempotencyConflict(t *testing.T) {
	server := makeServer(t)
	body1 := bytes.NewBufferString(`{"text":"contact jane@example.com","idempotency_key":"scan-idem-conflict"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/v1/scan", body1)
	req1.Header.Set("Content-Type", "application/json")
	resp1 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp1, req1)
	if resp1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.Code)
	}

	body2 := bytes.NewBufferString(`{"text":"different text with no pii","idempotency_key":"scan-idem-conflict"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/scan", body2)
	req2.Header.Set("Content-Type", "application/json")
	resp2 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp2, req2)
	assertJSONError(t, resp2, http.StatusConflict, "idempotency_conflict")
}

func TestTransformIdempotentReplay(t *testing.T) {
	server := makeServer(t)
	body1 := bytes.NewBufferString(`{"text":"contact jane@example.com","mode":"mask","idempotency_key":"transform-idem-1","request_id":"r1"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/v1/transform", body1)
	req1.Header.Set("Content-Type", "application/json")
	resp1 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp1, req1)
	if resp1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.Code)
	}
	var first models.TransformResponse
	if err := json.NewDecoder(resp1.Body).Decode(&first); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	body2 := bytes.NewBufferString(`{"text":"contact jane@example.com","mode":"mask","idempotency_key":"transform-idem-1","request_id":"r2"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/transform", body2)
	req2.Header.Set("Content-Type", "application/json")
	resp2 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.Code)
	}
	var second models.TransformResponse
	if err := json.NewDecoder(resp2.Body).Decode(&second); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if first.Stats.EntitiesTransformed != second.Stats.EntitiesTransformed {
		t.Fatalf("expected identical transformed entity count")
	}
	if first.Stats.ModesApplied != second.Stats.ModesApplied {
		t.Fatalf("expected identical modes applied")
	}
}

func TestTransformIdempotencyConflict(t *testing.T) {
	server := makeServer(t)
	body1 := bytes.NewBufferString(`{"text":"contact jane@example.com","mode":"mask","idempotency_key":"transform-idem-conflict"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/v1/transform", body1)
	req1.Header.Set("Content-Type", "application/json")
	resp1 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp1, req1)
	if resp1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.Code)
	}

	body2 := bytes.NewBufferString(`{"text":"different text","mode":"mask","idempotency_key":"transform-idem-conflict"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/transform", body2)
	req2.Header.Set("Content-Type", "application/json")
	resp2 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp2, req2)
	assertJSONError(t, resp2, http.StatusConflict, "idempotency_conflict")
}

func TestAnonymizeIdempotentReplay(t *testing.T) {
	server := makeServer(t)
	body1 := bytes.NewBufferString(`{"text":"contact jane@example.com","idempotency_key":"anon-idem-1","request_id":"r1"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/v1/anonymize", body1)
	req1.Header.Set("Content-Type", "application/json")
	resp1 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp1, req1)
	if resp1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.Code)
	}
	var first models.TransformResponse
	if err := json.NewDecoder(resp1.Body).Decode(&first); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	body2 := bytes.NewBufferString(`{"text":"contact jane@example.com","idempotency_key":"anon-idem-1","request_id":"r2"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/anonymize", body2)
	req2.Header.Set("Content-Type", "application/json")
	resp2 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp2, req2)
	if resp2.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.Code)
	}
	var second models.TransformResponse
	if err := json.NewDecoder(resp2.Body).Decode(&second); err != nil {
		t.Fatalf("decode failed: %v", err)
	}

	if first.Stats.EntitiesTransformed != second.Stats.EntitiesTransformed {
		t.Fatalf("expected identical transformed entity count")
	}
	if first.Output != second.Output {
		t.Fatalf("expected identical anonymized output")
	}
}

func TestAnonymizeIdempotencyConflict(t *testing.T) {
	server := makeServer(t)
	body1 := bytes.NewBufferString(`{"text":"contact jane@example.com","idempotency_key":"anon-idem-conflict"}`)
	req1 := httptest.NewRequest(http.MethodPost, "/v1/anonymize", body1)
	req1.Header.Set("Content-Type", "application/json")
	resp1 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp1, req1)
	if resp1.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp1.Code)
	}

	body2 := bytes.NewBufferString(`{"text":"another contact john@example.com","idempotency_key":"anon-idem-conflict"}`)
	req2 := httptest.NewRequest(http.MethodPost, "/v1/anonymize", body2)
	req2.Header.Set("Content-Type", "application/json")
	resp2 := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp2, req2)
	assertJSONError(t, resp2, http.StatusConflict, "idempotency_conflict")
}

func TestValidateMethodAndBadInputs(t *testing.T) {
	server := makeServer(t)
	t.Run("method_not_allowed", func(t *testing.T) {
		tests := []struct {
			name       string
			method     string
			path       string
			wantStatus int
		}{
			{name: "health", method: http.MethodPost, path: "/health", wantStatus: http.StatusMethodNotAllowed},
			{name: "policy_version", method: http.MethodPost, path: "/v1/policy/version", wantStatus: http.StatusMethodNotAllowed},
			{name: "scan", method: http.MethodGet, path: "/v1/scan", wantStatus: http.StatusMethodNotAllowed},
			{name: "decide", method: http.MethodGet, path: "/v1/decide", wantStatus: http.StatusMethodNotAllowed},
			{name: "transform", method: http.MethodGet, path: "/v1/transform", wantStatus: http.StatusMethodNotAllowed},
			{name: "anonymize", method: http.MethodGet, path: "/v1/anonymize", wantStatus: http.StatusMethodNotAllowed},
			{name: "receipts", method: http.MethodPost, path: "/v1/receipts/abc", wantStatus: http.StatusMethodNotAllowed},
		}

		for _, tc := range tests {
			t.Run(tc.name, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, tc.path, nil)
				resp := httptest.NewRecorder()
				server.Handler.ServeHTTP(resp, req)
				assertJSONError(t, resp, tc.wantStatus, "method_not_allowed")
			})
		}
	})

	t.Run("bad_request_payloads", func(t *testing.T) {
		scanReq := httptest.NewRequest(http.MethodPost, "/v1/scan", bytes.NewBufferString(`{"text":""}`))
		scanReq.Header.Set("Content-Type", "application/json")
		scanResp := httptest.NewRecorder()
		server.Handler.ServeHTTP(scanResp, scanReq)
		assertJSONError(t, scanResp, http.StatusBadRequest, "invalid_request")

		decideReq := httptest.NewRequest(http.MethodPost, "/v1/decide", bytes.NewBufferString(`{"action":{"type":""},"text":"jane@example.com"}`))
		decideReq.Header.Set("Content-Type", "application/json")
		decideResp := httptest.NewRecorder()
		server.Handler.ServeHTTP(decideResp, decideReq)
		assertJSONError(t, decideResp, http.StatusBadRequest, "invalid_request")

		transformReq := httptest.NewRequest(http.MethodPost, "/v1/transform", bytes.NewBufferString(`{"text":""}`))
		transformReq.Header.Set("Content-Type", "application/json")
		transformResp := httptest.NewRecorder()
		server.Handler.ServeHTTP(transformResp, transformReq)
		assertJSONError(t, transformResp, http.StatusBadRequest, "invalid_request")

		anonymizeReq := httptest.NewRequest(http.MethodPost, "/v1/anonymize", bytes.NewBufferString(`{"text":""}`))
		anonymizeReq.Header.Set("Content-Type", "application/json")
		anonymizeResp := httptest.NewRecorder()
		server.Handler.ServeHTTP(anonymizeResp, anonymizeReq)
		assertJSONError(t, anonymizeResp, http.StatusBadRequest, "invalid_request")
	})

	t.Run("missing_receipt", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/receipts/does-not-exist", nil)
		resp := httptest.NewRecorder()
		server.Handler.ServeHTTP(resp, req)
		assertJSONError(t, resp, http.StatusNotFound, "not_found")

		req = httptest.NewRequest(http.MethodGet, "/v1/receipts/", nil)
		resp = httptest.NewRecorder()
		server.Handler.ServeHTTP(resp, req)
		assertJSONError(t, resp, http.StatusNotFound, "not_found")
	})
}

func TestInvalidJSONHandling(t *testing.T) {
	server := makeServer(t)

	req := httptest.NewRequest(http.MethodPost, "/v1/scan", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)
	assertJSONError(t, resp, http.StatusBadRequest, "invalid_request")

	req = httptest.NewRequest(http.MethodPost, "/v1/decide", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)
	assertJSONError(t, resp, http.StatusBadRequest, "invalid_request")

	req = httptest.NewRequest(http.MethodPost, "/v1/transform", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)
	assertJSONError(t, resp, http.StatusBadRequest, "invalid_request")

	req = httptest.NewRequest(http.MethodPost, "/v1/anonymize", bytes.NewBufferString(`{`))
	req.Header.Set("Content-Type", "application/json")
	resp = httptest.NewRecorder()
	server.Handler.ServeHTTP(resp, req)
	assertJSONError(t, resp, http.StatusBadRequest, "invalid_request")
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

func assertJSONError(t *testing.T, resp *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if resp.Code != status {
		t.Fatalf("expected %d, got %d", status, resp.Code)
	}
	var got struct {
		Error models.APIError `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode error body failed: %v", err)
	}
	if got.Error.Code != code {
		t.Fatalf("expected error code %q, got %q", code, got.Error.Code)
	}
}
