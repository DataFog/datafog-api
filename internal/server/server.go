package server

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/datafog/datafog-api/internal/models"
	"github.com/datafog/datafog-api/internal/policy"
	"github.com/datafog/datafog-api/internal/receipts"
	"github.com/datafog/datafog-api/internal/scan"
	"github.com/datafog/datafog-api/internal/shim"
	"github.com/datafog/datafog-api/internal/transform"
)

type Server struct {
	policy      models.Policy
	store       *receipts.ReceiptStore
	eventReader shim.EventReader
	apiToken    string
	rateLimiter *tokenBucket
	startedAt   time.Time
	logger      *log.Logger
	mu          sync.Mutex
	statsMu     sync.Mutex
	decisions   map[string]idempotentDecision
	scans       map[string]idempotentCachedResponse
	transforms  map[string]idempotentCachedResponse
	anonymizes  map[string]idempotentCachedResponse
	totalCount  int64
	errorCount  int64
	statusHits  map[int]int64
	pathHits    map[string]int64
	methodHits  map[string]int64
}

type requestIDContextKey struct{}

type responseStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *responseStatusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *responseStatusWriter) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(body)
}

const (
	maxRequestBodyBytes int64 = 1024 * 1024 // 1 MiB
)

type idempotentDecision struct {
	requestHash string
	response    models.DecideResponse
}

type idempotentCachedResponse struct {
	requestHash string
	body        []byte
	status      int
}

type metricsResponse struct {
	TotalRequests int64            `json:"total_requests"`
	ErrorRequests int64            `json:"error_requests"`
	ByStatus      map[string]int64 `json:"by_status"`
	ByPath        map[string]int64 `json:"by_path"`
	ByMethod      map[string]int64 `json:"by_method"`
	StartedAt     string           `json:"started_at"`
	UptimeSeconds float64          `json:"uptime_seconds"`
}

func New(policyData models.Policy, store *receipts.ReceiptStore, logger *log.Logger, apiToken string, rateLimitRPS int) *Server {
	if logger == nil {
		logger = log.Default()
	}
	return &Server{
		policy:      policyData,
		store:       store,
		apiToken:    apiToken,
		rateLimiter: newTokenBucket(rateLimitRPS),
		startedAt:   time.Now().UTC(),
		logger:      logger,
		decisions:   map[string]idempotentDecision{},
		scans:       map[string]idempotentCachedResponse{},
		transforms:  map[string]idempotentCachedResponse{},
		anonymizes:  map[string]idempotentCachedResponse{},
		statusHits:  map[int]int64{},
		pathHits:    map[string]int64{},
		methodHits:  map[string]int64{},
	}
}

func (s *Server) SetEventReader(reader shim.EventReader) {
	s.eventReader = reader
}

// HandlerWithDemo returns the HTTP handler with optional demo endpoints registered.
func (s *Server) HandlerWithDemo(demo *DemoHandler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", s.handleHealth)
	mux.HandleFunc("/v1/policy/version", s.handlePolicyVersion)
	mux.HandleFunc("/v1/scan", s.handleScan)
	mux.HandleFunc("/v1/decide", s.handleDecide)
	mux.HandleFunc("/v1/transform", s.handleTransform)
	mux.HandleFunc("/v1/anonymize", s.handleAnonymize)
	mux.HandleFunc("/v1/receipts/", s.handleReceipt)
	mux.HandleFunc("/v1/events", s.handleEvents)
	mux.HandleFunc("/metrics", s.handleMetrics)
	if demo != nil {
		demo.Register(mux)
	}
	return s.wrapMiddleware(mux)
}

func (s *Server) Handler() http.Handler {
	return s.HandlerWithDemo(nil)
}

func (s *Server) wrapMiddleware(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-API-Key, X-Request-ID")
			w.Header().Set("Access-Control-Expose-Headers", "X-Request-ID")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		reqID := requestID(r)
		if reqID == "" {
			reqID = newRequestID()
		}
		r = r.WithContext(context.WithValue(r.Context(), requestIDContextKey{}, reqID))
		w.Header().Set("X-Request-ID", reqID)

		responseWriter := &responseStatusWriter{ResponseWriter: w}
		startedAt := time.Now()
		handler, pattern := mux.Handler(r)
		defer func() {
			if rec := recover(); rec != nil {
				responseWriter.status = http.StatusInternalServerError
				s.logger.Printf("request panic request_id=%s method=%s path=%s err=%v", reqID, r.Method, r.URL.Path, rec)
				s.respondError(responseWriter, http.StatusInternalServerError, models.APIError{Code: "internal_error", Message: "internal server error", RequestID: reqID})
			}
			if responseWriter.status == 0 {
				responseWriter.status = http.StatusOK
			}
			if pattern == "" {
				s.recordRequestMetrics(r.Method, "/_not_found", responseWriter.status)
			} else {
				s.recordRequestMetrics(r.Method, canonicalizedRoute(pattern, r.URL.Path), responseWriter.status)
			}
			s.logger.Printf("request complete request_id=%s method=%s path=%s status=%d latency_ms=%d", reqID, r.Method, r.URL.Path, responseWriter.status, time.Since(startedAt).Milliseconds())
		}()

		if !s.authorized(r) {
			s.respondError(responseWriter, http.StatusUnauthorized, models.APIError{Code: "unauthorized", Message: "missing or invalid API token", RequestID: reqID})
			return
		}
		if !s.rateLimiter.allow() {
			s.respondError(responseWriter, http.StatusTooManyRequests, models.APIError{Code: "rate_limited", Message: "request rate limit exceeded", RequestID: reqID})
			return
		}

		if pattern == "" {
			s.respondError(responseWriter, http.StatusNotFound, models.APIError{Code: "not_found", Message: "endpoint not found", RequestID: reqID})
			return
		}
		handler.ServeHTTP(responseWriter, r)
	})
}

func (s *Server) authorized(r *http.Request) bool {
	if s.apiToken == "" {
		return true
	}

	if token := authorizationToken(r.Header.Get("Authorization")); token != "" && constantTimeTokenEqual(token, s.apiToken) {
		return true
	}

	if token := strings.TrimSpace(r.Header.Get("X-API-Key")); token != "" && constantTimeTokenEqual(token, s.apiToken) {
		return true
	}

	return false
}

type tokenBucket struct {
	mu       sync.Mutex
	tokens   float64
	rate     float64
	capacity float64
	lastTime time.Time
}

func newTokenBucket(rateLimit int) *tokenBucket {
	if rateLimit <= 0 {
		return nil
	}

	rate := float64(rateLimit)
	return &tokenBucket{
		tokens:   rate,
		rate:     rate,
		capacity: rate,
		lastTime: time.Now(),
	}
}

func (tb *tokenBucket) allow() bool {
	if tb == nil {
		return true
	}

	tb.mu.Lock()
	defer tb.mu.Unlock()

	now := time.Now()
	delta := now.Sub(tb.lastTime).Seconds()
	if delta > 0 {
		tb.tokens = math.Min(tb.capacity, tb.tokens+(delta*tb.rate))
		tb.lastTime = now
	}
	if tb.tokens < 1 {
		return false
	}
	tb.tokens--
	return true
}

func authorizationToken(value string) string {
	parts := strings.Fields(strings.TrimSpace(value))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return parts[1]
}

func constantTimeTokenEqual(provided, expected string) bool {
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

func canonicalizedRoute(pattern string, path string) string {
	if strings.HasSuffix(pattern, "/") && strings.HasPrefix(path, "/v1/receipts/") {
		return "/v1/receipts/{id}"
	}
	return pattern
}

func (s *Server) recordRequestMetrics(method string, route string, status int) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	s.totalCount++
	s.methodHits[method]++
	s.pathHits[route]++
	s.statusHits[status]++
	if status >= 400 {
		s.errorCount++
	}
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
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		s.respondError(w, http.StatusUnsupportedMediaType, models.APIError{Code: "unsupported_media_type", Message: "content-type must be application/json", RequestID: requestID(r)})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req models.ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondErrorFromDecodeErr(w, r, err)
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
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		s.respondError(w, http.StatusUnsupportedMediaType, models.APIError{Code: "unsupported_media_type", Message: "content-type must be application/json", RequestID: requestID(r)})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req models.DecideRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondErrorFromDecodeErr(w, r, err)
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
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		s.respondError(w, http.StatusUnsupportedMediaType, models.APIError{Code: "unsupported_media_type", Message: "content-type must be application/json", RequestID: requestID(r)})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req models.TransformRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondErrorFromDecodeErr(w, r, err)
		return
	}
	if req.Text == "" {
		s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "text is required", RequestID: requestID(r)})
		return
	}
	if req.Mode != "" {
		req.Mode = models.TransformMode(strings.ToLower(strings.TrimSpace(string(req.Mode))))
		if !isAllowedTransformMode(req.Mode) {
			s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "unsupported transform mode", RequestID: requestID(r)})
			return
		}
	}
	if len(req.EntityModes) > 0 {
		canonicalModes := make(map[string]models.TransformMode, len(req.EntityModes))
		for entityType, mode := range req.EntityModes {
			entityType = strings.TrimSpace(entityType)
			if entityType == "" {
				s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "entity_modes keys must not be empty", RequestID: requestID(r)})
				return
			}
			canonicalMode := models.TransformMode(strings.ToLower(strings.TrimSpace(string(mode))))
			if !isAllowedTransformMode(canonicalMode) {
				s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "unsupported transform mode", RequestID: requestID(r)})
				return
			}
			canonicalModes[entityType] = canonicalMode
		}
		req.EntityModes = canonicalModes
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
	if !isJSONContentType(r.Header.Get("Content-Type")) {
		s.respondError(w, http.StatusUnsupportedMediaType, models.APIError{Code: "unsupported_media_type", Message: "content-type must be application/json", RequestID: requestID(r)})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBodyBytes)

	var req models.AnonymizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondErrorFromDecodeErr(w, r, err)
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

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be GET", RequestID: requestID(r)})
		return
	}
	if s.eventReader == nil {
		s.respond(w, http.StatusOK, map[string]interface{}{"events": []shim.DecisionEvent{}, "total": 0})
		return
	}

	q := shim.EventQuery{Limit: 100}
	if after := r.URL.Query().Get("after"); after != "" {
		if t, err := time.Parse(time.RFC3339, after); err == nil {
			q.After = &t
		}
	}
	if before := r.URL.Query().Get("before"); before != "" {
		if t, err := time.Parse(time.RFC3339, before); err == nil {
			q.Before = &t
		}
	}
	if decision := r.URL.Query().Get("decision"); decision != "" {
		q.Decision = decision
	}
	if adapter := r.URL.Query().Get("adapter"); adapter != "" {
		q.Adapter = adapter
	}
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if n, err := strconv.Atoi(limitStr); err == nil && n > 0 && n <= 1000 {
			q.Limit = n
		}
	}

	events, err := s.eventReader.Query(q)
	if err != nil {
		s.respondError(w, http.StatusInternalServerError, models.APIError{Code: "events_read_error", Message: err.Error(), RequestID: requestID(r)})
		return
	}
	if events == nil {
		events = []shim.DecisionEvent{}
	}
	s.respond(w, http.StatusOK, map[string]interface{}{"events": events, "total": len(events)})
}

func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		s.respondError(w, http.StatusMethodNotAllowed, models.APIError{Code: "method_not_allowed", Message: "method must be GET", RequestID: requestID(r)})
		return
	}
	metrics := s.snapshotMetrics()
	s.respond(w, http.StatusOK, metrics)
}

func (s *Server) snapshotMetrics() metricsResponse {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	byStatus := map[string]int64{}
	for status, count := range s.statusHits {
		byStatus[strconv.Itoa(status)] = count
	}

	byPath := map[string]int64{}
	for path, count := range s.pathHits {
		byPath[path] = count
	}

	byMethod := map[string]int64{}
	for method, count := range s.methodHits {
		byMethod[method] = count
	}

	return metricsResponse{
		TotalRequests: s.totalCount,
		ErrorRequests: s.errorCount,
		ByStatus:      byStatus,
		ByPath:        byPath,
		ByMethod:      byMethod,
		StartedAt:     s.startedAt.Format(time.RFC3339),
		UptimeSeconds: time.Since(s.startedAt).Seconds(),
	}
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

func isAllowedTransformMode(mode models.TransformMode) bool {
	switch mode {
	case models.TransformModeMask, models.TransformModeTokenize, models.TransformModeAnonymize, models.TransformModeRedact, models.TransformModeReplace, models.TransformModeHash:
		return true
	default:
		return false
	}
}

func newRequestID() string {
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		return fmt.Sprintf("rid-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(id)
}

func (s *Server) respondError(w http.ResponseWriter, status int, errResp models.APIError) {
	if errResp.Code == "" {
		errResp.Code = "error"
	}
	s.respond(w, status, map[string]models.APIError{"error": errResp})
}

func requestID(r *http.Request) string {
	if rid, ok := r.Context().Value(requestIDContextKey{}).(string); ok && rid != "" {
		return rid
	}
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

func isJSONContentType(value string) bool {
	mediatype, _, err := mime.ParseMediaType(value)
	if err != nil {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(mediatype), "application/json")
}

func isRequestTooLarge(err error) bool {
	var maxBytesErr *http.MaxBytesError
	return errors.As(err, &maxBytesErr)
}

func (s *Server) respondErrorFromDecodeErr(w http.ResponseWriter, r *http.Request, err error) {
	if isRequestTooLarge(err) {
		s.respondError(w, http.StatusRequestEntityTooLarge, models.APIError{Code: "request_too_large", Message: "request body exceeds limit", RequestID: requestID(r)})
		return
	}
	s.respondError(w, http.StatusBadRequest, models.APIError{Code: "invalid_request", Message: "invalid JSON body", Details: err.Error(), RequestID: requestID(r)})
}
