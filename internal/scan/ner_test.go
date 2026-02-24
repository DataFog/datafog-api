package scan

import (
	"testing"
)

func TestScanNERDetectsPerson(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{"titled trigger", "Contact Mr. John Smith for details", "John Smith"},
		{"Dr trigger", "Refer to Dr. Jane Williams immediately", "Jane Williams"},
		{"common first name", "Meeting with Sarah Johnson tomorrow", "Sarah Johnson"},
		{"common first name 2", "Email from Michael Chen about project", "Michael Chen"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := ScanNER(tt.text, []string{"person"})
			found := false
			for _, f := range findings {
				if f.EntityType == "person" && f.Value == tt.expected {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected person %q in %q, got findings: %+v", tt.expected, tt.text, findings)
			}
		})
	}
}

func TestScanNERDetectsOrganization(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{"inc suffix", "Filed by Acme Inc. yesterday", "Acme Inc."},
		{"corp suffix", "Work for DataFog Corp in tech", "DataFog Corp"},
		{"llc suffix", "Founded Bright Solutions LLC last year", "Bright Solutions LLC"},
		{"university suffix", "Studied at Stanford University for years", "Stanford University"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := ScanNER(tt.text, []string{"organization"})
			found := false
			for _, f := range findings {
				if f.EntityType == "organization" && f.Value == tt.expected {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected organization %q in %q, got findings: %+v", tt.expected, tt.text, findings)
			}
		})
	}
}

func TestScanNERDetectsLocation(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{"dictionary city", "Office in San Francisco downtown", "San Francisco"},
		{"dictionary city 2", "Moved to New York for work", "New York"},
		{"trigger word", "Lives in Portland with family", "Portland"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := ScanNER(tt.text, []string{"location"})
			found := false
			for _, f := range findings {
				if f.EntityType == "location" && f.Value == tt.expected {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected location %q in %q, got findings: %+v", tt.expected, tt.text, findings)
			}
		})
	}
}

func TestScanNERDisabledReturnsNothing(t *testing.T) {
	NEREnabled = false
	defer func() { NEREnabled = true }()

	findings := ScanNER("Contact Mr. John Smith at Acme Corp in New York", nil)
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when NER disabled, got %d: %+v", len(findings), findings)
	}
}

func TestScanNERFiltering(t *testing.T) {
	text := "Mr. John Smith works at Acme Corp in New York"

	// Only request person
	findings := ScanNER(text, []string{"person"})
	for _, f := range findings {
		if f.EntityType != "person" {
			t.Fatalf("expected only person findings, got %q", f.EntityType)
		}
	}
}

func TestScanNERIntegration(t *testing.T) {
	// Test that ScanText includes NER results alongside regex results
	text := "Contact Mr. John Smith at john@example.com or 555-123-4567"
	findings := ScanText(text, nil)

	foundTypes := map[string]bool{}
	for _, f := range findings {
		foundTypes[f.EntityType] = true
	}

	if !foundTypes["person"] {
		t.Error("expected person entity from NER")
	}
	if !foundTypes["email"] {
		t.Error("expected email entity from regex")
	}
	if !foundTypes["phone"] {
		t.Error("expected phone entity from regex")
	}
}
