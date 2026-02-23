package policy

import (
	"os"
	"strings"
	"testing"

	"github.com/datafog/datafog-api/internal/models"
)

func basePolicy() models.Policy {
	return models.Policy{
		PolicyID:      "mvp",
		PolicyVersion: "v1",
		Rules: []models.Rule{
			{
				ID:       "deny-api-key-shell",
				Priority: 100,
				Effect:   models.DecisionDeny,
				Match: models.MatchCriteria{
					ActionTypes: []string{"shell.exec"},
				},
				EntityRequirements: []string{"api_key"},
			},
			{
				ID:       "transform-sensitive",
				Priority: 90,
				Effect:   models.DecisionTransform,
				Match: models.MatchCriteria{
					ActionTypes: []string{"file.write", "http.request", "shell.exec"},
				},
				EntityRequirements: []string{"email"},
			},
			{
				ID:       "allow-safe",
				Priority: 10,
				Effect:   models.DecisionAllow,
				Match: models.MatchCriteria{
					ActionTypes: []string{"file.read", "shell.exec"},
				},
			},
		},
	}
}

func TestEvaluateDenyOnAPIKeyForShell(t *testing.T) {
	policy := basePolicy()
	ctx := DecisionContext{
		Action:   models.ActionMeta{Type: "shell.exec", Resource: "curl"},
		Findings: []models.ScanFinding{{EntityType: "api_key", Value: "ABC1234567890123", Start: 0, End: 16, Confidence: .9}},
	}
	result := Evaluate(policy, ctx)
	if result.Decision != models.DecisionDeny {
		t.Fatalf("expected deny, got %s", result.Decision)
	}
	if result.MatchedRules[0] != "deny-api-key-shell" {
		t.Fatalf("expected deny rule match, got %v", result.MatchedRules)
	}
}

func TestEvaluateTransformWhenSensitiveEntity(t *testing.T) {
	policy := basePolicy()
	ctx := DecisionContext{
		Action:   models.ActionMeta{Type: "file.write", Resource: "notes.txt"},
		Findings: []models.ScanFinding{{EntityType: "email", Value: "a@b.com", Start: 0, End: 7, Confidence: .98}},
	}
	result := Evaluate(policy, ctx)
	if result.Decision != models.DecisionTransform {
		t.Fatalf("expected transform, got %s", result.Decision)
	}
	if len(result.TransformPlan) == 0 {
		t.Fatalf("expected transform plan")
	}
}

func TestEvaluateAllowWhenNoSensitiveEntity(t *testing.T) {
	policy := basePolicy()
	ctx := DecisionContext{
		Action:   models.ActionMeta{Type: "file.read", Resource: "notes.txt"},
		Findings: []models.ScanFinding{},
	}
	result := Evaluate(policy, ctx)
	if result.Decision != models.DecisionAllow {
		t.Fatalf("expected allow, got %s", result.Decision)
	}
}

func TestEvaluateDefaultDenyForUnknownAction(t *testing.T) {
	policy := basePolicy()
	result := Evaluate(policy, DecisionContext{Action: models.ActionMeta{Type: "unknown.action"}})
	if result.Decision != models.DecisionDeny {
		t.Fatalf("expected deny for unknown action, got %s", result.Decision)
	}
}

func TestEvaluateDenylRuleAlwaysWins(t *testing.T) {
	policy := models.Policy{
		PolicyID:      "mvp",
		PolicyVersion: "v1",
		Rules: []models.Rule{
			{
				ID:       "transform-low-priority",
				Priority: 90,
				Effect:   models.DecisionTransform,
				Match: models.MatchCriteria{
					ActionTypes: []string{"file.write"},
				},
				EntityRequirements: []string{"email"},
			},
			{
				ID:       "deny-low-priority",
				Priority: 10,
				Effect:   models.DecisionDeny,
				Match: models.MatchCriteria{
					ActionTypes: []string{"file.write"},
				},
				EntityRequirements: []string{"api_key"},
			},
		},
	}
	result := Evaluate(policy, DecisionContext{
		Action: models.ActionMeta{Type: "file.write", Resource: "notes.txt"},
		Findings: []models.ScanFinding{
			{EntityType: "email", Value: "a@b.com", Start: 0, End: 7, Confidence: .98},
			{EntityType: "api_key", Value: "ABCD1234EFGH5678", Start: 9, End: 25, Confidence: .98},
		},
	})
	if result.Decision != models.DecisionDeny {
		t.Fatalf("expected deny to take precedence, got %s", result.Decision)
	}
	if len(result.MatchedRules) != 2 {
		t.Fatalf("expected 2 matched rules, got %v", result.MatchedRules)
	}
}

func TestEvaluateTransformBeatsRedaction(t *testing.T) {
	policy := models.Policy{
		PolicyID:      "mvp",
		PolicyVersion: "v1",
		Rules: []models.Rule{
			{
				ID:       "redact-mid-priority",
				Priority: 50,
				Effect:   models.DecisionAllowWithRedaction,
				Match: models.MatchCriteria{
					ActionTypes: []string{"file.write"},
				},
			},
			{
				ID:       "transform-high-priority",
				Priority: 40,
				Effect:   models.DecisionTransform,
				Match: models.MatchCriteria{
					ActionTypes: []string{"file.write"},
				},
				EntityRequirements: []string{"email"},
			},
		},
	}
	result := Evaluate(policy, DecisionContext{
		Action: models.ActionMeta{Type: "file.write", Resource: "notes.txt"},
		Findings: []models.ScanFinding{
			{EntityType: "email", Value: "a@b.com", Start: 0, End: 7, Confidence: .98},
		},
	})
	if result.Decision != models.DecisionTransform {
		t.Fatalf("expected transform to beat allow_with_redaction, got %s", result.Decision)
	}
	if len(result.TransformPlan) == 0 {
		t.Fatalf("expected transform plan for transform decision")
	}
}

func TestValidatePolicyRejectsUnknownEffect(t *testing.T) {
	policy := basePolicy()
	policy.Rules[0].Effect = models.Decision("unsupported")
	if err := ValidatePolicy(policy); err == nil {
		t.Fatal("expected validation error")
	} else if !strings.Contains(err.Error(), "unsupported effect") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidatePolicyRejectsDuplicateRuleIDs(t *testing.T) {
	policy := basePolicy()
	policy.Rules[1].ID = policy.Rules[0].ID
	if err := ValidatePolicy(policy); err == nil {
		t.Fatal("expected duplicate rule id error")
	} else if !strings.Contains(err.Error(), "duplicate rule id") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidatePolicyRejectsUnsupportedEntityRequirement(t *testing.T) {
	policy := basePolicy()
	policy.Rules[0].EntityRequirements = []string{"not_a_real_entity"}
	if err := ValidatePolicy(policy); err == nil {
		t.Fatal("expected unsupported entity requirement error")
	} else if !strings.Contains(err.Error(), "unsupported required entity type") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidatePolicyRejectsInvalidTransformMode(t *testing.T) {
	policy := basePolicy()
	policy.Rules[1].EntityTransforms = []models.TransformStep{{EntityType: "email", Mode: "invalid"}}
	if err := ValidatePolicy(policy); err == nil {
		t.Fatal("expected invalid transform mode error")
	} else if !strings.Contains(err.Error(), "unsupported transform mode") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidatePolicyRejectsUnsupportedTransformEntityType(t *testing.T) {
	policy := basePolicy()
	policy.Rules[1].EntityTransforms = []models.TransformStep{{EntityType: "not_real", Mode: models.TransformModeMask}}
	if err := ValidatePolicy(policy); err == nil {
		t.Fatal("expected unsupported transform entity type error")
	} else if !strings.Contains(err.Error(), "unsupported transform entity type") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidatePolicyRejectsEmptyRuleID(t *testing.T) {
	policy := basePolicy()
	policy.Rules[0].ID = " "
	if err := ValidatePolicy(policy); err == nil {
		t.Fatal("expected missing rule id error")
	} else if !strings.Contains(err.Error(), "rule missing id") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

func TestValidatePolicyRejectsEmptyMatchEntries(t *testing.T) {
	policy := basePolicy()
	policy.Rules[0].Match.ActionTypes = []string{""}
	policy.Rules[0].Match.ResourcePrefix = []string{" "}
	if err := ValidatePolicy(policy); err == nil {
		t.Fatal("expected empty match criteria error")
	} else if !strings.Contains(err.Error(), "empty action_type condition") && !strings.Contains(err.Error(), "empty resource_prefix condition") {
		t.Fatalf("unexpected validation error: %v", err)
	}
}

type policyDecisionVector struct {
	Name              string               `json:"name"`
	Action            models.ActionMeta    `json:"action"`
	Findings          []models.ScanFinding `json:"findings"`
	ExpectedDecision  models.Decision      `json:"expected_decision"`
	ExpectedRules     []string             `json:"expected_matched_rules"`
	ExpectedTransform bool                 `json:"expected_transform"`
}

func TestEvaluateGoldenPolicyVectors(t *testing.T) {
	vectors := []policyDecisionVector{
		{
			Name:             "allow read action",
			Action:           models.ActionMeta{Type: "file.read", Resource: "notes.txt"},
			ExpectedDecision: models.DecisionAllow,
			ExpectedRules:    []string{"allow-safe"},
		},
		{
			Name:              "transform file write with email",
			Action:            models.ActionMeta{Type: "file.write", Resource: "notes.txt"},
			Findings:          []models.ScanFinding{{EntityType: "email", Value: "jane@x.com", Start: 0, End: 9, Confidence: 0.99}},
			ExpectedDecision:  models.DecisionTransform,
			ExpectedRules:     []string{"transform-sensitive"},
			ExpectedTransform: true,
		},
		{
			Name:             "deny shell with api key",
			Action:           models.ActionMeta{Type: "shell.exec", Resource: "curl"},
			Findings:         []models.ScanFinding{{EntityType: "api_key", Value: "ABCD1234EFGH5678", Start: 0, End: 16, Confidence: 0.99}},
			ExpectedDecision: models.DecisionDeny,
			ExpectedRules:    []string{"deny-api-key-shell", "allow-safe"},
		},
		{
			Name:             "default deny for unmatched",
			Action:           models.ActionMeta{Type: "unknown.action"},
			ExpectedDecision: models.DecisionDeny,
		},
	}

	policy := basePolicy()

	for _, vector := range vectors {
		t.Run(vector.Name, func(t *testing.T) {
			t.Parallel()
			result := Evaluate(policy, DecisionContext{Action: vector.Action, Findings: vector.Findings})
			if result.Decision != vector.ExpectedDecision {
				t.Fatalf("expected decision %q, got %q", vector.ExpectedDecision, result.Decision)
			}
			if len(vector.ExpectedRules) > 0 {
				if len(result.MatchedRules) != len(vector.ExpectedRules) {
					t.Fatalf("expected %d matched rules, got %v", len(vector.ExpectedRules), result.MatchedRules)
				}
				for idx := range vector.ExpectedRules {
					if result.MatchedRules[idx] != vector.ExpectedRules[idx] {
						t.Fatalf("expected rule %q at %d, got %v", vector.ExpectedRules[idx], idx, result.MatchedRules)
					}
				}
			}
			if vector.ExpectedTransform && len(result.TransformPlan) == 0 {
				t.Fatalf("expected transform plan")
			}
		})
	}
}

func TestLoadPolicyFromFileRejectsMissingMetadata(t *testing.T) {
	policyPath := t.TempDir() + "/policy.json"
	policy := `{"rules":[{"id":"allow-read","priority":1,"effect":"allow","match":{"action_types":["file.read"]}}]}`
	if err := os.WriteFile(policyPath, []byte(policy), 0o644); err != nil {
		t.Fatalf("seed policy file failed: %v", err)
	}

	if _, err := LoadPolicyFromFile(policyPath); err == nil {
		t.Fatal("expected policy load error")
	}
}
