package policy

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/datafog/datafog-api/internal/models"
)

var RequiredDecisionInputs = map[models.Decision]struct{}{
	models.DecisionAllow:              {},
	models.DecisionDeny:               {},
	models.DecisionTransform:          {},
	models.DecisionAllowWithRedaction: {},
}

func LoadPolicyFromFile(path string) (models.Policy, error) {
	var policy models.Policy
	content, err := os.ReadFile(path)
	if err != nil {
		return policy, err
	}
	if err := json.Unmarshal(content, &policy); err != nil {
		return policy, err
	}
	if err := ValidatePolicy(policy); err != nil {
		return policy, err
	}
	return policy, nil
}

func ValidatePolicy(policy models.Policy) error {
	errors := make([]string, 0)
	if strings.TrimSpace(policy.PolicyID) == "" {
		errors = append(errors, "policy_id is required")
	}
	if strings.TrimSpace(policy.PolicyVersion) == "" {
		errors = append(errors, "policy_version is required")
	}
	if len(policy.Rules) == 0 {
		errors = append(errors, "policy must contain at least one rule")
	}

	seenRuleIDs := map[string]struct{}{}
	for _, rule := range policy.Rules {
		ruleID := strings.TrimSpace(rule.ID)
		if ruleID == "" {
			errors = append(errors, "rule missing id")
			continue
		}
		if _, ok := seenRuleIDs[ruleID]; ok {
			errors = append(errors, fmt.Sprintf("duplicate rule id: %s", ruleID))
		}
		seenRuleIDs[ruleID] = struct{}{}
		if rule.Priority < 0 {
			errors = append(errors, fmt.Sprintf("rule %s has negative priority: %d", ruleID, rule.Priority))
		}
		if _, ok := RequiredDecisionInputs[rule.Effect]; !ok {
			errors = append(errors, fmt.Sprintf("rule %s has unsupported effect: %s", ruleID, rule.Effect))
		}
		for _, actionType := range rule.Match.ActionTypes {
			if strings.TrimSpace(actionType) == "" {
				errors = append(errors, fmt.Sprintf("rule %s has empty action_type condition", ruleID))
			}
		}
		for _, tool := range rule.Match.Tools {
			if strings.TrimSpace(tool) == "" {
				errors = append(errors, fmt.Sprintf("rule %s has empty tool condition", ruleID))
			}
		}
		for _, prefix := range rule.Match.ResourcePrefix {
			if strings.TrimSpace(prefix) == "" {
				errors = append(errors, fmt.Sprintf("rule %s has empty resource_prefix condition", ruleID))
			}
		}
		for _, requirement := range rule.EntityRequirements {
			reqName := strings.ToLower(strings.TrimSpace(requirement))
			if reqName == "" {
				errors = append(errors, fmt.Sprintf("rule %s has empty entity_requirement", ruleID))
				continue
			}
			if _, ok := defaultEntityTypes[reqName]; !ok {
				errors = append(errors, fmt.Sprintf("rule %s references unsupported required entity type: %s", ruleID, requirement))
			}
		}
		for _, step := range rule.EntityTransforms {
			if strings.TrimSpace(step.EntityType) == "" {
				errors = append(errors, fmt.Sprintf("rule %s has entity transform without entity_type", ruleID))
				continue
			}
			entityType := strings.ToLower(strings.TrimSpace(step.EntityType))
			if _, ok := defaultEntityTypes[entityType]; !ok {
				errors = append(errors, fmt.Sprintf("rule %s references unsupported transform entity type: %s", ruleID, step.EntityType))
			}
			if _, ok := allowedModes[step.Mode]; !ok {
				errors = append(errors, fmt.Sprintf("rule %s references unsupported transform mode: %s", ruleID, step.Mode))
			}
		}
	}

	if len(errors) == 0 {
		return nil
	}
	return fmt.Errorf(strings.Join(errors, "; "))
}

var allowedModes = map[models.TransformMode]struct{}{
	models.TransformModeMask:      {},
	models.TransformModeTokenize:  {},
	models.TransformModeAnonymize: {},
	models.TransformModeRedact:    {},
}

type DecisionContext struct {
	Action   models.ActionMeta
	Findings []models.ScanFinding
}

type DecisionResult struct {
	Decision      models.Decision
	MatchedRules  []string
	TransformPlan []models.TransformStep
	Reason        string
}

var defaultEntityTransforms = []models.TransformStep{
	{EntityType: "email", Mode: models.TransformModeMask},
	{EntityType: "phone", Mode: models.TransformModeTokenize},
	{EntityType: "ssn", Mode: models.TransformModeAnonymize},
	{EntityType: "api_key", Mode: models.TransformModeRedact},
	{EntityType: "credit_card", Mode: models.TransformModeRedact},
}

var defaultEntityTypes = map[string]struct{}{
	"email":       {},
	"phone":       {},
	"ssn":         {},
	"api_key":     {},
	"credit_card": {},
}

func Evaluate(policy models.Policy, ctx DecisionContext) DecisionResult {
	if ctx.Action.Type == "" {
		return DecisionResult{
			Decision: models.DecisionDeny,
			Reason:   "action.type is required",
		}
	}

	if len(policy.Rules) == 0 {
		return DecisionResult{
			Decision: models.DecisionDeny,
			Reason:   "policy has no rules",
		}
	}

	rules := append([]models.Rule(nil), policy.Rules...)
	sort.SliceStable(rules, func(i, j int) bool {
		return rules[i].Priority > rules[j].Priority
	})

	hasFindings := map[string]struct{}{}
	for _, f := range ctx.Findings {
		hasFindings[strings.ToLower(f.EntityType)] = struct{}{}
	}

	matchIDs := []string{}
	transformPlan := []models.TransformStep{}
	transformFound := false
	transformWithRedaction := false
	denyReason := ""
	denyMatched := false

	matched := false
	for _, rule := range rules {
		if _, ok := RequiredDecisionInputs[rule.Effect]; !ok {
			continue
		}
		if !matchAction(rule.Match, ctx.Action) {
			continue
		}
		if !hasRequiredEntities(rule.EntityRequirements, hasFindings) {
			continue
		}
		matched = true
		matchIDs = append(matchIDs, rule.ID)

		switch rule.Effect {
		case models.DecisionDeny:
			denyMatched = true
			if denyReason == "" {
				denyReason = rule.Description
			}
		case models.DecisionTransform:
			transformFound = true
			if len(rule.EntityTransforms) > 0 {
				transformPlan = append(transformPlan, rule.EntityTransforms...)
			}
		case models.DecisionAllowWithRedaction:
			transformWithRedaction = true
		}
	}

	if denyMatched {
		return DecisionResult{
			Decision:      models.DecisionDeny,
			MatchedRules:  matchIDs,
			TransformPlan: nil,
			Reason:        denyReason,
		}
	}
	if transformFound {
		if len(transformPlan) == 0 {
			transformPlan = defaultEntityTransforms
		}
		return DecisionResult{
			Decision:      models.DecisionTransform,
			MatchedRules:  matchIDs,
			TransformPlan: transformPlan,
		}
	}
	if transformWithRedaction {
		if len(transformPlan) == 0 {
			transformPlan = defaultEntityTransforms
		}
		return DecisionResult{
			Decision:      models.DecisionAllowWithRedaction,
			MatchedRules:  matchIDs,
			TransformPlan: transformPlan,
		}
	}
	if matched {
		return DecisionResult{
			Decision:     models.DecisionAllow,
			MatchedRules: matchIDs,
		}
	}

	return DecisionResult{
		Decision: models.DecisionDeny,
		Reason:   "no matching rule",
	}
}

func matchAction(match models.MatchCriteria, action models.ActionMeta) bool {
	if !matchesField(match.ActionTypes, action.Type) {
		return false
	}
	if !matchesField(match.Tools, action.Tool) {
		return false
	}
	if len(match.ResourcePrefix) > 0 && action.Resource == "" {
		return false
	}
	for _, prefix := range match.ResourcePrefix {
		if strings.HasPrefix(action.Resource, prefix) {
			return true
		}
	}
	if len(match.ResourcePrefix) > 0 {
		return false
	}
	return true
}

func matchesField(allowed []string, value string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, allow := range allowed {
		if strings.EqualFold(allow, value) {
			return true
		}
	}
	return false
}

func hasRequiredEntities(reqs []string, found map[string]struct{}) bool {
	for _, req := range reqs {
		reqName := strings.ToLower(req)
		if _, ok := defaultEntityTypes[reqName]; !ok {
			return false
		}
		if _, ok := found[reqName]; !ok {
			return false
		}
	}
	return true
}

func (res DecisionResult) String() string {
	return fmt.Sprintf("%s decision=%s matched=%v reason=%s", res.Decision, res.Decision, res.MatchedRules, res.Reason)
}
