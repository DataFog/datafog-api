package transform

import "testing"

import "github.com/datafog/datafog-api/internal/models"

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
