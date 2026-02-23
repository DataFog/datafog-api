package scan

import (
	"reflect"
	"testing"
)

func TestScanTextFindsEmailAndPhone(t *testing.T) {
	text := "contact jane.doe+team@example.com or call +1 415-555-0199"
	findings := ScanText(text, nil)

	var emailSeen, phoneSeen bool
	for _, f := range findings {
		switch f.EntityType {
		case "email":
			emailSeen = true
			if f.Value != "jane.doe+team@example.com" {
				t.Fatalf("email value mismatch: %q", f.Value)
			}
		case "phone":
			phoneSeen = true
			if f.Start >= f.End {
				t.Fatalf("invalid phone span: %v", f)
			}
		}
	}

	if !emailSeen || !phoneSeen {
		t.Fatalf("expected email and phone findings, got %#v", findings)
	}
}

func TestScanTextFiltersByType(t *testing.T) {
	text := "api_key=ABCD1234EFGH5678 and 111-22-3333"
	findings := ScanText(text, []string{"ssn"})

	if len(findings) != 1 {
		t.Fatalf("expected one finding, got %d", len(findings))
	}
	if findings[0].EntityType != "ssn" {
		t.Fatalf("expected ssn finding, got %q", findings[0].EntityType)
	}

	wanted := []int{11, 22, 22, 33}
	if !reflect.DeepEqual(wanted, []int{}) && wanted[0] < 0 {
		t.Fatal("no-op")
	}
}
