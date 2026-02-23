package models

import "time"

type Decision string

const (
	DecisionAllow              Decision = "allow"
	DecisionDeny               Decision = "deny"
	DecisionTransform          Decision = "transform"
	DecisionAllowWithRedaction Decision = "allow_with_redaction"
)

type TransformMode string

const (
	TransformModeMask      TransformMode = "mask"
	TransformModeTokenize  TransformMode = "tokenize"
	TransformModeAnonymize TransformMode = "anonymize"
	TransformModeRedact    TransformMode = "redact"
)

type ScanFinding struct {
	EntityType string  `json:"entity_type"`
	Value      string  `json:"value"`
	Start      int     `json:"start"`
	End        int     `json:"end"`
	Confidence float64 `json:"confidence"`
}

type ScanRequest struct {
	Text           string   `json:"text"`
	EntityTypes    []string `json:"entity_types,omitempty"`
	RequestID      string   `json:"request_id,omitempty"`
	TraceID        string   `json:"trace_id,omitempty"`
	IdempotencyKey string   `json:"idempotency_key,omitempty"`
}

type ScanResponse struct {
	RequestID     string        `json:"request_id"`
	TraceID       string        `json:"trace_id,omitempty"`
	Findings      []ScanFinding `json:"findings"`
	PolicyVersion string        `json:"policy_version"`
	PolicyID      string        `json:"policy_id"`
}

type ActionMeta struct {
	Type      string   `json:"type"`
	Tool      string   `json:"tool,omitempty"`
	Resource  string   `json:"resource,omitempty"`
	Command   string   `json:"command,omitempty"`
	Args      []string `json:"args,omitempty"`
	Sensitive bool     `json:"sensitive,omitempty"`
}

type DecideRequest struct {
	RequestID      string        `json:"request_id,omitempty"`
	TraceID        string        `json:"trace_id,omitempty"`
	TenantID       string        `json:"tenant_id,omitempty"`
	ActorID        string        `json:"actor_id,omitempty"`
	SessionID      string        `json:"session_id,omitempty"`
	Action         ActionMeta    `json:"action"`
	Text           string        `json:"text,omitempty"`
	Findings       []ScanFinding `json:"findings,omitempty"`
	IdempotencyKey string        `json:"idempotency_key,omitempty"`
}

type DecideResponse struct {
	RequestID     string          `json:"request_id,omitempty"`
	TraceID       string          `json:"trace_id,omitempty"`
	Decision      Decision        `json:"decision"`
	ReceiptID     string          `json:"receipt_id"`
	PolicyVersion string          `json:"policy_version"`
	PolicyID      string          `json:"policy_id"`
	MatchedRules  []string        `json:"matched_rules"`
	TransformPlan []TransformStep `json:"transform_plan,omitempty"`
	Findings      []ScanFinding   `json:"findings"`
	Reason        string          `json:"reason,omitempty"`
}

type TransformRequest struct {
	Text           string                   `json:"text"`
	Findings       []ScanFinding            `json:"findings,omitempty"`
	Mode           TransformMode            `json:"mode,omitempty"`
	EntityModes    map[string]TransformMode `json:"entity_modes,omitempty"`
	RequestID      string                   `json:"request_id,omitempty"`
	TraceID        string                   `json:"trace_id,omitempty"`
	IdempotencyKey string                   `json:"idempotency_key,omitempty"`
}

type TransformResponse struct {
	RequestID     string         `json:"request_id"`
	TraceID       string         `json:"trace_id,omitempty"`
	Output        string         `json:"output"`
	PolicyID      string         `json:"policy_id"`
	PolicyVersion string         `json:"policy_version"`
	Stats         TransformStats `json:"stats"`
}

type TransformStep struct {
	EntityType string        `json:"entity_type"`
	Mode       TransformMode `json:"mode"`
}

type TransformStats struct {
	EntitiesTransformed int    `json:"entities_transformed"`
	ModesApplied        string `json:"modes_applied"`
}

type AnonymizeRequest struct {
	Text           string        `json:"text"`
	Findings       []ScanFinding `json:"findings,omitempty"`
	RequestID      string        `json:"request_id,omitempty"`
	TraceID        string        `json:"trace_id,omitempty"`
	IdempotencyKey string        `json:"idempotency_key,omitempty"`
}

type PolicyRequestContext struct {
	PolicyID      string
	PolicyVersion string
	ActiveRules   []Rule
}

type HealthResponse struct {
	Status        string `json:"status"`
	PolicyID      string `json:"policy_id"`
	PolicyVersion string `json:"policy_version"`
	StartedAt     string `json:"started_at"`
}

type Receipt struct {
	ReceiptID        string          `json:"receipt_id"`
	Timestamp        time.Time       `json:"timestamp"`
	RequestID        string          `json:"request_id"`
	TraceID          string          `json:"trace_id"`
	TenantID         string          `json:"tenant_id"`
	ActorID          string          `json:"actor_id"`
	SessionID        string          `json:"session_id"`
	PolicyVersion    string          `json:"policy_version"`
	PolicyID         string          `json:"policy_id"`
	ActionHash       string          `json:"action_hash"`
	InputHash        string          `json:"input_hash"`
	SanitizedSummary string          `json:"sanitized_summary,omitempty"`
	Decision         Decision        `json:"decision"`
	Action           ActionMeta      `json:"action"`
	MatchedRules     []string        `json:"matched_rules"`
	Findings         []ScanFinding   `json:"findings"`
	TransformPlan    []TransformStep `json:"transform_plan,omitempty"`
	Reason           string          `json:"reason,omitempty"`
}

type APIError struct {
	Code      string `json:"code"`
	Message   string `json:"message"`
	RequestID string `json:"request_id,omitempty"`
	Details   string `json:"details,omitempty"`
}

type MatchCriteria struct {
	ActionTypes    []string `json:"action_types,omitempty"`
	Tools          []string `json:"tools,omitempty"`
	ResourcePrefix []string `json:"resource_prefixes,omitempty"`
	Commands       []string `json:"commands,omitempty"`
	Args           []string `json:"args,omitempty"`
}

type Rule struct {
	ID                   string          `json:"id"`
	Description          string          `json:"description"`
	Priority             int             `json:"priority"`
	Effect               Decision        `json:"effect"`
	Match                MatchCriteria   `json:"match"`
	EntityRequirements   []string        `json:"entity_requirements,omitempty"`
	EntityTransforms     []TransformStep `json:"entity_transforms,omitempty"`
	RequireSensitiveOnly bool            `json:"require_sensitive_only,omitempty"`
}

type Policy struct {
	PolicyID      string    `json:"policy_id"`
	PolicyVersion string    `json:"policy_version"`
	Description   string    `json:"description,omitempty"`
	UpdatedAt     time.Time `json:"updated_at"`
	Rules         []Rule    `json:"rules"`
}
