package policy

import (
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
