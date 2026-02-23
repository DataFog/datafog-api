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
	if policy.PolicyID == "" {
		policy.PolicyID = "default"
	}
	if policy.PolicyVersion == "" {
		policy.PolicyVersion = "0001"
	}
	if err := ValidatePolicy(policy); err != nil {
		return policy, err
	}
	return policy, nil
}

func ValidatePolicy(policy models.Policy) error {
	errors := make([]string, 0)
	if len(policy.Rules) == 0 {
		errors = append(errors, "policy must contain at least one rule")
	}

	seenRuleIDs := map[string]struct{}{}
	for _, rule := range policy.Rules {
		if rule.ID == "" {
			errors = append(errors, "rule missing id")
		}
		if _, ok := seenRuleIDs[rule.ID]; ok {
			errors = append(errors, fmt.Sprintf("duplicate rule id: %s", rule.ID))
		}
		seenRuleIDs[rule.ID] = struct{}{}
		if _, ok := RequiredDecisionInputs[rule.Effect]; !ok {
			errors = append(errors, fmt.Sprintf("rule %s has unsupported effect: %s", rule.ID, rule.Effect))
		}
		for _, requirement := range rule.EntityRequirements {
			if _, ok := defaultEntityTypes[strings.ToLower(requirement)]; !ok {
				errors = append(errors, fmt.Sprintf("rule %s references unsupported required entity type: %s", rule.ID, requirement))
			}
		}
	}

	if len(errors) == 0 {
		return nil
	}
	return fmt.Errorf(strings.Join(errors, "; "))
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
			if denyReason == "" {
				denyReason = rule.Description
			}
			return DecisionResult{
				Decision:      models.DecisionDeny,
				MatchedRules:  matchIDs,
				TransformPlan: nil,
				Reason:        denyReason,
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
			continue
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
