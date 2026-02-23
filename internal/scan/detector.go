package scan

import (
	"sort"
	"regexp"
	"strings"

	"github.com/datafog/datafog-api/internal/models"
)

var DefaultEntityPatterns = map[string]*regexp.Regexp{
	"email":       regexp.MustCompile(`(?i)\b[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}\b`),
	"phone":       regexp.MustCompile(`(?:\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`),
	"ssn":         regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	"api_key":     regexp.MustCompile(`(?i)\b(?:apikey|api[_-]?key|token)[:=]\s*[a-zA-Z0-9]{16,64}\b`),
	"credit_card": regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`),
}

var DefaultEntityConfidences = map[string]float64{
	"email":       0.995,
	"phone":       0.88,
	"ssn":         0.99,
	"api_key":     0.94,
	"credit_card": 0.9,
}

func ScanText(text string, entityFilter []string) []models.ScanFinding {
	requested := map[string]struct{}{}
	if len(entityFilter) > 0 {
		for _, name := range entityFilter {
			requested[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
		}
	}

	findings := make([]models.ScanFinding, 0)
	entityTypes := make([]string, 0, len(DefaultEntityPatterns))
	for entityType := range DefaultEntityPatterns {
		entityTypes = append(entityTypes, entityType)
	}
	sort.Strings(entityTypes)

	for _, entityType := range entityTypes {
		re := DefaultEntityPatterns[entityType]
		if len(requested) > 0 {
			if _, ok := requested[entityType]; !ok {
				continue
			}
		}

		idxs := re.FindAllStringIndex(text, -1)
		for _, idx := range idxs {
			if len(idx) != 2 || idx[0] < 0 || idx[1] < idx[0] {
				continue
			}
			value := text[idx[0]:idx[1]]
			findings = append(findings, models.ScanFinding{
				EntityType: entityType,
				Value:      value,
				Start:      idx[0],
				End:        idx[1],
				Confidence: DefaultEntityConfidences[entityType],
			})
		}
	}

	return findings
}
