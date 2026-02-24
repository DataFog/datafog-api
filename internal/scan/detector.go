package scan

import (
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/datafog/datafog-api/internal/models"
)

// EntityPattern pairs a compiled regex with an optional validator.
// If Validate is non-nil it is called on every regex match; only matches
// that return true are kept.  This lets us do cheap regex first, then
// expensive checks (Luhn, IP-range) only on candidates.
type EntityPattern struct {
	Re       *regexp.Regexp
	Validate func(match string) bool
}

// DefaultEntityPatterns maps entity type names to their detection patterns.
var DefaultEntityPatterns = map[string]EntityPattern{
	"email": {
		Re: regexp.MustCompile(`(?i)\b[\w.+-]+@[\w.-]+\.[A-Za-z]{2,}\b`),
	},
	"phone": {
		Re: regexp.MustCompile(`(?:\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}`),
	},
	"ssn": {
		Re: regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`),
	},
	"api_key": {
		Re: regexp.MustCompile(`(?i)\b(?:apikey|api[_-]?key|token)[:=]\s*[a-zA-Z0-9]{16,64}\b`),
	},
	"credit_card": {
		Re:       regexp.MustCompile(`\b(?:\d[ -]*?){13,19}\b`),
		Validate: luhnValid,
	},
	"ip_address": {
		Re:       regexp.MustCompile(`\b(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})\b`),
		Validate: ipv4Valid,
	},
	"date": {
		Re: regexp.MustCompile(
			`\b(?:` +
				`\d{4}[-/]\d{1,2}[-/]\d{1,2}` + // YYYY-MM-DD or YYYY/MM/DD
				`|` +
				`\d{1,2}[-/]\d{1,2}[-/]\d{2,4}` + // MM/DD/YYYY, DD/MM/YYYY, MM-DD-YY
				`|` +
				`(?:Jan|Feb|Mar|Apr|May|Jun|Jul|Aug|Sep|Oct|Nov|Dec)[a-z]*\.?\s+\d{1,2},?\s+\d{4}` + // Month DD, YYYY
				`)\b`,
		),
	},
	"zip_code": {
		Re: regexp.MustCompile(`\b\d{5}(?:-\d{4})?\b`),
	},
}

var DefaultEntityConfidences = map[string]float64{
	"email":       0.995,
	"phone":       0.88,
	"ssn":         0.99,
	"api_key":     0.94,
	"credit_card": 0.95, // higher now with Luhn validation
	"ip_address":  0.90,
	"date":        0.85,
	"zip_code":    0.80,
}

func ScanText(text string, entityFilter []string) []models.ScanFinding {
	requested := map[string]struct{}{}
	if len(entityFilter) > 0 {
		for _, name := range entityFilter {
			requested[strings.ToLower(strings.TrimSpace(name))] = struct{}{}
		}
	}

	findings := make([]models.ScanFinding, 0)

	// Phase 1: Regex engine (fast, always available)
	entityTypes := make([]string, 0, len(DefaultEntityPatterns))
	for entityType := range DefaultEntityPatterns {
		entityTypes = append(entityTypes, entityType)
	}
	sort.Strings(entityTypes)

	for _, entityType := range entityTypes {
		pattern := DefaultEntityPatterns[entityType]
		if len(requested) > 0 {
			if _, ok := requested[entityType]; !ok {
				continue
			}
		}

		idxs := pattern.Re.FindAllStringIndex(text, -1)
		for _, idx := range idxs {
			if len(idx) != 2 || idx[0] < 0 || idx[1] < idx[0] {
				continue
			}
			value := text[idx[0]:idx[1]]
			if pattern.Validate != nil && !pattern.Validate(value) {
				continue
			}
			findings = append(findings, models.ScanFinding{
				EntityType: entityType,
				Value:      value,
				Start:      idx[0],
				End:        idx[1],
				Confidence: DefaultEntityConfidences[entityType],
			})
		}
	}

	// Phase 2: NER engine (heuristic, when enabled)
	nerFindings := ScanNER(text, entityFilter)
	findings = append(findings, nerFindings...)

	return findings
}

// luhnValid implements the Luhn algorithm to validate credit card numbers.
// It strips spaces and dashes before checking.
func luhnValid(s string) bool {
	// Strip spaces and dashes
	var digits []int
	for _, ch := range s {
		if ch >= '0' && ch <= '9' {
			digits = append(digits, int(ch-'0'))
		} else if ch == ' ' || ch == '-' {
			continue
		} else {
			return false
		}
	}
	if len(digits) < 13 || len(digits) > 19 {
		return false
	}

	sum := 0
	double := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := digits[i]
		if double {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		double = !double
	}
	return sum%10 == 0
}

// ipv4Valid checks that each octet is 0-255.
func ipv4Valid(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 4 {
		return false
	}
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil {
			return false
		}
		if n < 0 || n > 255 {
			return false
		}
	}
	return true
}
