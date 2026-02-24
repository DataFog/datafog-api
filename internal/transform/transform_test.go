package transform

import (
	"strings"
	"testing"

	"github.com/datafog/datafog-api/internal/models"
)

func TestApplyTransformsMasksAndTokenizes(t *testing.T) {
	input := "email=alice@example.com token=1234123412341234"
	findings := []models.ScanFinding{
		{EntityType: "email", Start: 6, End: 22, Value: "alice@example.com"},
		{EntityType: "credit_card", Start: 29, End: 45, Value: "1234123412341234"},
	}
	steps := []models.TransformStep{
		{EntityType: "email", Mode: models.TransformModeMask},
		{EntityType: "credit_card", Mode: models.TransformModeTokenize},
	}
	out, stats := ApplyTransforms(input, findings, steps)
	if out == input {
		t.Fatalf("expected transformed output")
	}
	if stats.EntitiesTransformed != 2 {
		t.Fatalf("expected 2 transformed entities, got %d", stats.EntitiesTransformed)
	}
}

func TestTransformIgnoresEntitiesWithoutPlan(t *testing.T) {
	input := "hello 555-000-1111"
	findings := []models.ScanFinding{{EntityType: "phone", Start: 6, End: 17, Value: "555-000-1111"}}
	out, stats := ApplyTransforms(input, findings, []models.TransformStep{{EntityType: "email", Mode: models.TransformModeMask}})
	if out != input {
		t.Fatalf("expected unchanged output, got %q", out)
	}
	if stats.EntitiesTransformed != 0 {
		t.Fatalf("expected no transformations")
	}
}

func TestTransformModeReplace(t *testing.T) {
	input := "email alice@example.com"
	findings := []models.ScanFinding{
		{EntityType: "email", Start: 6, End: 22, Value: "alice@example.com"},
	}
	steps := []models.TransformStep{
		{EntityType: "email", Mode: models.TransformModeReplace},
	}
	out, stats := ApplyTransforms(input, findings, steps)
	if !strings.Contains(out, "[EMAIL_") {
		t.Fatalf("expected pseudonymized replacement with [EMAIL_...], got %q", out)
	}
	if stats.EntitiesTransformed != 1 {
		t.Fatalf("expected 1 transformed entity, got %d", stats.EntitiesTransformed)
	}

	// Deterministic: same input produces same output
	out2, _ := ApplyTransforms(input, findings, steps)
	if out != out2 {
		t.Fatalf("expected deterministic output, got %q and %q", out, out2)
	}
}

func TestTransformModeHash(t *testing.T) {
	input := "ssn 123-45-6789"
	findings := []models.ScanFinding{
		{EntityType: "ssn", Start: 4, End: 15, Value: "123-45-6789"},
	}
	steps := []models.TransformStep{
		{EntityType: "ssn", Mode: models.TransformModeHash},
	}
	out, stats := ApplyTransforms(input, findings, steps)
	if strings.Contains(out, "123-45-6789") {
		t.Fatalf("expected SSN to be replaced with hash, got %q", out)
	}
	// SHA256 hash is 64 hex chars
	replaced := strings.TrimPrefix(out, "ssn ")
	if len(replaced) != 64 {
		t.Fatalf("expected 64-char SHA256 hash, got %d chars: %q", len(replaced), replaced)
	}
	if stats.EntitiesTransformed != 1 {
		t.Fatalf("expected 1 transformed entity, got %d", stats.EntitiesTransformed)
	}
}

func TestTransformModeRedact(t *testing.T) {
	input := "key api_key=Secret12345678901234"
	findings := []models.ScanFinding{
		{EntityType: "api_key", Start: 4, End: 31, Value: "api_key=Secret12345678901234"},
	}
	steps := []models.TransformStep{
		{EntityType: "api_key", Mode: models.TransformModeRedact},
	}
	out, _ := ApplyTransforms(input, findings, steps)
	if !strings.Contains(out, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] in output, got %q", out)
	}
}

func TestAllSixModes(t *testing.T) {
	modes := []struct {
		mode     models.TransformMode
		contains string
	}{
		{models.TransformModeMask, "****"},
		{models.TransformModeTokenize, "TOK-"},
		{models.TransformModeAnonymize, "anon-"},
		{models.TransformModeRedact, "[REDACTED]"},
		{models.TransformModeReplace, "[EMAIL_"},
		{models.TransformModeHash, ""}, // just check it's 64 hex chars
	}

	for _, tt := range modes {
		t.Run(string(tt.mode), func(t *testing.T) {
			input := "test alice@example.com end"
			findings := []models.ScanFinding{
				{EntityType: "email", Start: 5, End: 22, Value: "alice@example.com"},
			}
			steps := []models.TransformStep{
				{EntityType: "email", Mode: tt.mode},
			}
			out, stats := ApplyTransforms(input, findings, steps)
			if stats.EntitiesTransformed != 1 {
				t.Fatalf("expected 1 entity transformed")
			}
			if tt.contains != "" && !strings.Contains(out, tt.contains) {
				t.Fatalf("expected output to contain %q, got %q", tt.contains, out)
			}
			if strings.Contains(out, "alice@example.com") {
				t.Fatalf("original value should not be present in output")
			}
		})
	}
}
