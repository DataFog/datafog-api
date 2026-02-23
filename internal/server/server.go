package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/datafog/datafog-api/internal/models"
	"github.com/datafog/datafog-api/internal/policy"
	"github.com/datafog/datafog-api/internal/receipts"
	"github.com/datafog/datafog-api/internal/scan"
	"github.com/datafog/datafog-api/internal/transform"
)

type Server struct {
	policy     models.Policy
	store      *receipts.ReceiptStore
	startedAt  time.Time
	logger     *log.Logger
	mu         sync.Mutex
	decisions  map[string]idempotentDecision
	scans      map[string]idempotentCachedResponse
	transforms map[string]idempotentCachedResponse
	anonymizes map[string]idempotentCachedResponse
}

type idempotentDecision struct {
	requestHash string
	response    models.DecideResponse
}

type idempotentCachedResponse struct {
	requestHash string
	body        []byte
	status      int
}

func New(policyData models.Policy, store *receipts.ReceiptStore, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	return &Server{
		policy:     policyData,
		store:      store,
		startedAt:  time.Now().UTC(),
		logger:     logger,
		decisions:  map[string]idempotentDecision{},
		scans:      map[string]idempotentCachedResponse{},
		transforms: map[string]idempotentCachedResponse{},
		anonymizes: map[string]idempotentCachedResponse{},
	}
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/policy/version", s.handlePolicyVersion)
	mux.HandleFunc("/v1/scan", s.handleScan)
	mux.HandleFunc("/v1/decide", s.handleDecide)
	mux.HandleFunc("/v1/transform", s.handleTransform)
	mux.HandleFunc("/v1/anonymize", s.handleAnonymize)
	mux.HandleFunc("/v1/receipts/", s.handleReceipt)
	return mux
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be GET", RequestID: requestID(r)})
		return
	}
	res := models.HealthResponse{
		Status:        "ok",
		PolicyID:      s.policy.PolicyID,
		PolicyVersion: s.policy.PolicyVersion,
		StartedAt:     s.startedAt.Format(time.RFC3339),
	}
	s.respond(w, http.StatusOK, res)
}

func (s *Server) handlePolicyVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be GET", RequestID: requestID(r)})
		return
	}
	s.respond(w, http.StatusOK, map[string]string{
		"policy_id":      s.policy.PolicyID,
		"policy_version": s.policy.PolicyVersion,
	})
}

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be POST", RequestID: requestID(r)})
		return
	}

	var req models.ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "invalid JSON body", Details: err.Error(), RequestID: requestID(r)})
		return
	}
	if req.Text == "" {
		s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "text is required", RequestID: requestID(r)})
		return
	}
	if req.IdempotencyKey != "" {
		reqHash, err := hashScanRequest(req)
		if err != nil {
			s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "unable to hash request payload", Details: err.Error(), RequestID: requestID(r)})
			return
		}
		s.mu.Lock()
		existing, ok := s.scans[req.IdempotencyKey]
		s.mu.Unlock()
		if ok {
			if existing.requestHash != reqHash {
				s.respondError(w, http.StatusConflict, models.APIError{Code: "idempotency_conflict", Message: "different request payload for same idempotency_key", RequestID: requestID(r)})
				return
			}
			s.respondRaw(w, existing.status, existing.body)
			return
		}
	}

	findings := scan.ScanText(req.Text, req.EntityTypes)
	res := models.ScanResponse{
		RequestID:     req.RequestID,
		TraceID:       req.TraceID,
		Findings:      findings,
		PolicyVersion: s.policy.PolicyVersion,
		PolicyID:      s.policy.PolicyID,
	}
	if req.IdempotencyKey != "" {
		body, err := json.Marshal(res)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, models.APIError{Code: "encode_error", Message: "unable to encode response", Details: err.Error(), RequestID: requestID(r)})
			return
		}
		hash, _ := hashScanRequest(req)
		s.mu.Lock()
		s.scans[req.IdempotencyKey] = idempotentCachedResponse{
			requestHash: hash,
			body:        body,
			status:      http.StatusOK,
		}
		s.mu.Unlock()
		s.respondRaw(w, http.StatusOK, body)
		return
	}
	s.respond(w, http.StatusOK, res)
}

func (s *Server) handleDecide(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be POST", RequestID: requestID(r)})
		return
	}

	var req models.DecideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "invalid JSON body", Details: err.Error(), RequestID: requestID(r)})
		return
	}
	if req.Action.Type == "" {
		s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "action.type is required", RequestID: requestID(r)})
		return
	}
	if req.IdempotencyKey != "" {
		reqHash, err := hashDecideRequest(req)
		if err != nil {
			s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "unable to hash request payload", Details: err.Error(), RequestID: requestID(r)})
			return
		}
		s.mu.Lock()
		existing, ok := s.decisions[req.IdempotencyKey]
		s.mu.Unlock()
		if ok {
			if existing.requestHash != reqHash {
				s.respondError(w, http.StatusConflict, models.APIError{Code: "idempotency_conflict", Message: "different request payload for same idempotency_key", RequestID: requestID(r)})
				return
			}
			s.respond(w, http.StatusOK, existing.response)
			return
		}
	}

	findings := req.Findings
	if len(findings) == 0 && req.Text != "" {
		findings = scan.ScanText(req.Text, nil)
	}
	result := policy.Evaluate(s.policy, policy.DecisionContext{Action: req.Action, Findings: findings})
	actionHash, err := hashDecideAction(req.Action)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, models.APIError{Code: "hash_error", Message: "unable to hash action", Details: err.Error(), RequestID: requestID(r)})
		return
	}
	inputHash, err := hashDecideInput(req)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, models.APIError{Code: "hash_error", Message: "unable to hash request input", Details: err.Error(), RequestID: requestID(r)})
		return
	}
	receipt := s.store.NewReceipt(req, result.Decision, result, s.policy)
	receipt.Findings = findings
	receipt.ActionHash = actionHash
	receipt.InputHash = inputHash
	if len(result.TransformPlan) > 0 {
		summary, err := json.Marshal(result.TransformPlan)
		if err == nil {
			receipt.SanitizedSummary = string(summary)
		} else {
			receipt.SanitizedSummary = `transform plan unavailable`
		}
	}
	saved, err := s.store.Save(receipt)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, models.APIError{Code: "receipt_error", Message: "unable to persist receipt", Details: err.Error(), RequestID: requestID(r)})
		return
	}

	res := models.DecideResponse{
		RequestID:     req.RequestID,
		TraceID:       req.TraceID,
		Decision:      result.Decision,
		ReceiptID:     saved.ReceiptID,
		PolicyVersion: s.policy.PolicyVersion,
		PolicyID:      s.policy.PolicyID,
		MatchedRules:  result.MatchedRules,
		TransformPlan: result.TransformPlan,
		Findings:      findings,
		Reason:        result.Reason,
	}
	if req.IdempotencyKey != "" {
		hash, _ := hashDecideRequest(req)
		s.mu.Lock()
		s.decisions[req.IdempotencyKey] = idempotentDecision{
			requestHash: hash,
			response:    res,
		}
		s.mu.Unlock()
	}
	s.respond(w, http.StatusOK, res)
}

func (s *Server) handleTransform(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be POST", RequestID: requestID(r)})
		return
	}
	var req models.TransformRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "invalid JSON body", Details: err.Error(), RequestID: requestID(r)})
		return
	}
	if req.Text == "" {
		s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "text is required", RequestID: requestID(r)})
		return
	}
	if req.IdempotencyKey != "" {
		reqHash, err := hashTransformRequest(req)
		if err != nil {
			s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "unable to hash request payload", Details: err.Error(), RequestID: requestID(r)})
			return
		}
		s.mu.Lock()
		existing, ok := s.transforms[req.IdempotencyKey]
		s.mu.Unlock()
		if ok {
			if existing.requestHash != reqHash {
				s.respondError(w, http.StatusConflict, models.APIError{Code: "idempotency_conflict", Message: "different request payload for same idempotency_key", RequestID: requestID(r)})
				return
			}
			s.respondRaw(w, existing.status, existing.body)
			return
		}
	}

	findings := req.Findings
	if len(findings) == 0 {
		findings = scan.ScanText(req.Text, nil)
	}

	entityModes := req.EntityModes
	if len(entityModes) == 0 {
		entityModes = map[string]models.TransformMode{}
		if req.Mode != "" {
			for _, f := range findings {
				entityModes[f.EntityType] = req.Mode
			}
		}
	}
	plan := make([]models.TransformStep, 0, len(entityModes))
	for entityType, mode := range entityModes {
		plan = append(plan, models.TransformStep{EntityType: entityType, Mode: mode})
	}

	output, stats := transform.ApplyTransforms(req.Text, findings, plan)
	res := models.TransformResponse{
		RequestID:     req.RequestID,
		TraceID:       req.TraceID,
		Output:        output,
		PolicyID:      s.policy.PolicyID,
		PolicyVersion: s.policy.PolicyVersion,
		Stats:         stats,
	}
	if req.IdempotencyKey != "" {
		body, err := json.Marshal(res)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, models.APIError{Code: "encode_error", Message: "unable to encode response", Details: err.Error(), RequestID: requestID(r)})
			return
		}
		hash, _ := hashTransformRequest(req)
		s.mu.Lock()
		s.transforms[req.IdempotencyKey] = idempotentCachedResponse{
			requestHash: hash,
			body:        body,
			status:      http.StatusOK,
		}
		s.mu.Unlock()
		s.respondRaw(w, http.StatusOK, body)
		return
	}
	s.respond(w, http.StatusOK, res)
}

func (s *Server) handleAnonymize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		s.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be POST", RequestID: requestID(r)})
		return
	}

	var req models.AnonymizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "invalid JSON body", Details: err.Error(), RequestID: requestID(r)})
		return
	}
	if req.Text == "" {
		s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "text is required", RequestID: requestID(r)})
		return
	}
	if req.IdempotencyKey != "" {
		reqHash, err := hashAnonymizeRequest(req)
		if err != nil {
			s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "unable to hash request payload", Details: err.Error(), RequestID: requestID(r)})
			return
		}
		s.mu.Lock()
		existing, ok := s.anonymizes[req.IdempotencyKey]
		s.mu.Unlock()
		if ok {
			if existing.requestHash != reqHash {
				s.respondError(w, http.StatusConflict, models.APIError{Code: "idempotency_conflict", Message: "different request payload for same idempotency_key", RequestID: requestID(r)})
				return
			}
			s.respondRaw(w, existing.status, existing.body)
			return
		}
	}

	findings := req.Findings
	if len(findings) == 0 {
		findings = scan.ScanText(req.Text, nil)
	}

	plan := make([]models.TransformStep, 0)
	seen := map[string]struct{}{}
	for _, f := range findings {
		if _, ok := seen[f.EntityType]; ok {
			continue
		}
		seen[f.EntityType] = struct{}{}
		plan = append(plan, models.TransformStep{EntityType: f.EntityType, Mode: models.TransformModeAnonymize})
	}

	output, stats := transform.ApplyTransforms(req.Text, findings, plan)
	res := models.TransformResponse{
		RequestID:     req.RequestID,
		TraceID:       req.TraceID,
		Output:        output,
		PolicyID:      s.policy.PolicyID,
		PolicyVersion: s.policy.PolicyVersion,
		Stats:         stats,
	}
	if req.IdempotencyKey != "" {
		body, err := json.Marshal(res)
		if err != nil {
			s.respondError(w, http.StatusInternalServerError, models.APIError{Code: "encode_error", Message: "unable to encode response", Details: err.Error(), RequestID: requestID(r)})
			return
		}
		hash, _ := hashAnonymizeRequest(req)
		s.mu.Lock()
		s.anonymizes[req.IdempotencyKey] = idempotentCachedResponse{
			requestHash: hash,
			body:        body,
			status:      http.StatusOK,
		}
		s.mu.Unlock()
		s.respondRaw(w, http.StatusOK, body)
		return
	}
	s.respond(w, http.StatusOK, res)
}

func (s *Server) handleReceipt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be GET", RequestID: requestID(r)})
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/v1/receipts/")
	if id == "" || strings.Contains(id, "/") {
		s.respondError(w, http.StatusNotFound, models.APIError{Code: "not_found", Message: "receipt id missing"})
		return
	}
	receipt, ok := s.store.Get(id)
	if !ok {
		s.respondError(w, http.StatusNotFound, models.APIError{Code: "not_found", Message: "receipt not found", RequestID: requestID(r)})
		return
	}
	s.respond(w, http.StatusOK, receipt)
}

func (s *Server) respond(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(payload); err != nil {
		s.logger.Printf("response encode failed: %v", err)
	}
}

func (s *Server) respondRaw(w http.ResponseWriter, status int, body []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(body); err != nil {
		s.logger.Printf("response write failed: %v", err)
	}
}

func (s *Server) respondError(w http.ResponseWriter, status int, errResp models.APIError) {
	if errResp.Code == "" {
		errResp.Code = "error"
	}
	s.respond(w, status, map[string]models.APIError{"error": errResp})
}

func requestID(r *http.Request) string {
	if rid := r.Header.Get("x-request-id"); rid != "" {
		return rid
	}
	return ""
}

func hashDecideRequest(req models.DecideRequest) (string, error) {
	req.IdempotencyKey = ""
	req.RequestID = ""
	req.TraceID = ""
	req.SessionID = ""
	req.ActorID = ""
	req.TenantID = ""
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func hashDecideAction(action models.ActionMeta) (string, error) {
	sum, err := hashPayload(action)
	if err != nil {
		return "", err
	}
	return sum, nil
}

func hashDecideInput(req models.DecideRequest) (string, error) {
	req.IdempotencyKey = ""
	req.RequestID = ""
	req.TraceID = ""
	req.SessionID = ""
	req.ActorID = ""
	req.TenantID = ""
	req.Action = models.ActionMeta{}
	return hashPayload(req)
}

func hashScanRequest(req models.ScanRequest) (string, error) {
	req.IdempotencyKey = ""
	req.RequestID = ""
	req.TraceID = ""
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func hashPayload(value interface{}) (string, error) {
	body, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func hashTransformRequest(req models.TransformRequest) (string, error) {
	req.IdempotencyKey = ""
	req.RequestID = ""
	req.TraceID = ""
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}

func hashAnonymizeRequest(req models.AnonymizeRequest) (string, error) {
	req.IdempotencyKey = ""
	req.RequestID = ""
	req.TraceID = ""
	body, err := json.Marshal(req)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), nil
}
