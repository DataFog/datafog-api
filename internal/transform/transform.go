package transform

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/datafog/datafog-api/internal/models"
)

func ApplyTransforms(input string, findings []models.ScanFinding, steps []models.TransformStep) (string, models.TransformStats) {
	if len(findings) == 0 {
		return input, models.TransformStats{}
	}

	modeMap := map[string]models.TransformMode{}
	for _, step := range steps {
		modeMap[step.EntityType] = step.Mode
	}

	filtered := make([]models.ScanFinding, 0, len(findings))
	for _, finding := range findings {
		if _, ok := modeMap[finding.EntityType]; !ok {
			continue
		}
		filtered = append(filtered, finding)
	}

	if len(filtered) == 0 {
		return input, models.TransformStats{}
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Start == filtered[j].Start {
			return filtered[i].End > filtered[j].End
		}
		return filtered[i].Start > filtered[j].Start
	})

	appliedModes := map[string]bool{}
	bytes := []byte(input)
	count := 0
	for _, finding := range filtered {
		if finding.Start < 0 || finding.End > len(bytes) || finding.Start >= finding.End {
			continue
		}
		replacement := replacementForMode(modeMap[finding.EntityType], finding.Value)
		before := bytes[:finding.Start]
		after := bytes[finding.End:]
		bytes = append(before, append([]byte(replacement), after...)...)
		appliedModes[fmt.Sprintf("%s:%s", finding.EntityType, modeMap[finding.EntityType])] = true
		count++
	}

	parts := make([]string, 0, len(appliedModes))
	for k := range appliedModes {
		parts = append(parts, k)
	}
	sort.Strings(parts)

	return string(bytes), models.TransformStats{
		EntitiesTransformed: count,
		ModesApplied:        strings.Join(parts, ","),
	}
}

func replacementForMode(mode models.TransformMode, value string) string {
	switch mode {
	case models.TransformModeTokenize:
		return "TOK-" + deterministicPrefix(value)
	case models.TransformModeAnonymize:
		return "anon-" + deterministicPrefix(value)
	case models.TransformModeRedact:
		return "[REDACTED]"
	case models.TransformModeMask:
		return strings.Repeat("*", len(value))
	default:
		return "[FILTERED]"
	}
}

func deterministicPrefix(value string) string {
	hash := sha256.Sum256([]byte(value))
	encoded := hex.EncodeToString(hash[:])
	return encoded[:8]
}
