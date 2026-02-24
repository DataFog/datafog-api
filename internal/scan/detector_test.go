package scan

import (
	"encoding/json"
	"reflect"
	"testing"
)

type scanGoldenCase struct {
	Name     string               `json:"name"`
	Text     string               `json:"text"`
	Filter   []string             `json:"filter"`
	Findings []FindingExpectation `json:"expected_findings"`
}

type FindingExpectation struct {
	EntityType string `json:"entity_type"`
	Start      int    `json:"start"`
	End        int    `json:"end"`
	Value      string `json:"value"`
}

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
	if findings[0].Start != 29 || findings[0].End != 40 {
		t.Fatalf("expected ssn span [29,40], got [%d,%d]", findings[0].Start, findings[0].End)
	}
}

func TestScanTextGoldenCorpus(t *testing.T) {
	data := []scanGoldenCase{
		{
			Name: "default email and phone",
			Text: "contact jane.doe@example.com on +1 415-555-0100",
			Findings: []FindingExpectation{
				{EntityType: "email", Start: 8, End: 28, Value: "jane.doe@example.com"},
				{EntityType: "phone", Start: 32, End: 47, Value: "+1 415-555-0100"},
			},
		},
		{
			Name:     "filter phone",
			Text:     "identity number 111-22-3333 and phone 555-123-4567",
			Filter:   []string{"phone"},
			Findings: []FindingExpectation{{EntityType: "phone", Start: 38, End: 50, Value: "555-123-4567"}},
		},
	}

	for _, vector := range data {
		t.Run(vector.Name, func(t *testing.T) {
			got := ScanText(vector.Text, vector.Filter)
			if len(got) != len(vector.Findings) {
				t.Fatalf("expected %d findings, got %d: %+v", len(vector.Findings), len(got), got)
			}

			for idx, exp := range vector.Findings {
				if got[idx].EntityType != exp.EntityType {
					t.Fatalf("expected %q at %d, got %q", exp.EntityType, idx, got[idx].EntityType)
				}
				if got[idx].Value != exp.Value || got[idx].Start != exp.Start || got[idx].End != exp.End {
					t.Fatalf("finding mismatch at %d: got %+v expected type=%s start=%d end=%d value=%q", idx, got[idx], exp.EntityType, exp.Start, exp.End, exp.Value)
				}
			}
		})
	}
}

func TestScanTextCorpusIsDeterministicWhenReloadedFromJSON(t *testing.T) {
	data := []scanGoldenCase{
		{
			Name: "cc detection",
			Text: "card 4111111111111111",
			Findings: []FindingExpectation{{
				EntityType: "credit_card",
				Start:      5,
				End:        21,
				Value:      "4111111111111111",
			}},
		},
	}

	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var vectors []scanGoldenCase
	if err := json.Unmarshal(raw, &vectors); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	first := ScanText(vectors[0].Text, nil)
	second := ScanText(vectors[0].Text, nil)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("expected deterministic results, got %+v and %+v", first, second)
	}
}

func TestScanTextNoPanicOnMalformedUTF8(t *testing.T) {
	malformed := string([]byte("contact "))
	malformed += string([]byte{0xff, 0xfe})
	malformed += " jane@example.com"

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("ScanText panicked on malformed UTF-8: %v", recovered)
		}
	}()

	_ = ScanText(malformed, nil)
}

// --- New entity type tests ---

func TestScanTextDetectsIPAddress(t *testing.T) {
	text := "server at 192.168.1.1 and gateway 10.0.0.1"
	findings := ScanText(text, []string{"ip_address"})

	if len(findings) != 2 {
		t.Fatalf("expected 2 ip_address findings, got %d: %+v", len(findings), findings)
	}
	if findings[0].Value != "192.168.1.1" {
		t.Fatalf("expected 192.168.1.1, got %q", findings[0].Value)
	}
	if findings[1].Value != "10.0.0.1" {
		t.Fatalf("expected 10.0.0.1, got %q", findings[1].Value)
	}
}

func TestScanTextRejectsInvalidIPAddress(t *testing.T) {
	text := "invalid ip 999.999.999.999 should not match"
	findings := ScanText(text, []string{"ip_address"})

	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for invalid IP, got %d: %+v", len(findings), findings)
	}
}

func TestScanTextDetectsDate(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		value string
	}{
		{"ISO format", "born on 1990-01-15 in city", "1990-01-15"},
		{"US slash", "due date 01/15/2025 payment", "01/15/2025"},
		{"US dash", "due date 01-15-2025 payment", "01-15-2025"},
		{"Month name", "born on January 15, 2025 in city", "January 15, 2025"},
		{"Month abbrev", "born on Jan 15, 2025 in city", "Jan 15, 2025"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			findings := ScanText(tt.text, []string{"date"})
			if len(findings) == 0 {
				t.Fatalf("expected date finding for %q, got none", tt.text)
			}
			found := false
			for _, f := range findings {
				if f.Value == tt.value {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("expected value %q in findings %+v", tt.value, findings)
			}
		})
	}
}

func TestScanTextDetectsZipCode(t *testing.T) {
	text := "address in 90210 or full zip 10001-1234"
	findings := ScanText(text, []string{"zip_code"})

	if len(findings) != 2 {
		t.Fatalf("expected 2 zip_code findings, got %d: %+v", len(findings), findings)
	}
	if findings[0].Value != "90210" {
		t.Fatalf("expected 90210, got %q", findings[0].Value)
	}
	if findings[1].Value != "10001-1234" {
		t.Fatalf("expected 10001-1234, got %q", findings[1].Value)
	}
}

func TestScanTextCreditCardLuhnValidation(t *testing.T) {
	// Valid Visa test number (passes Luhn)
	text := "card 4111111111111111 is valid"
	findings := ScanText(text, []string{"credit_card"})

	if len(findings) != 1 {
		t.Fatalf("expected 1 credit_card finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Value != "4111111111111111" {
		t.Fatalf("expected 4111111111111111, got %q", findings[0].Value)
	}

	// Invalid number (fails Luhn)
	textInvalid := "card 1234567890123456 is invalid"
	findingsInvalid := ScanText(textInvalid, []string{"credit_card"})
	if len(findingsInvalid) != 0 {
		t.Fatalf("expected 0 credit_card findings for invalid number, got %d: %+v", len(findingsInvalid), findingsInvalid)
	}
}

func TestLuhnValid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"Visa test", "4111111111111111", true},
		{"Mastercard test", "5500000000000004", true},
		{"Amex test", "378282246310005", true},
		{"With spaces", "4111 1111 1111 1111", true},
		{"With dashes", "4111-1111-1111-1111", true},
		{"Invalid", "1234567890123456", false},
		{"Too short", "123", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := luhnValid(tt.input)
			if got != tt.valid {
				t.Fatalf("luhnValid(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

func TestIPv4Valid(t *testing.T) {
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{"normal", "192.168.1.1", true},
		{"zeros", "0.0.0.0", true},
		{"max", "255.255.255.255", true},
		{"overflow", "256.1.1.1", false},
		{"overflow octet 4", "1.1.1.999", false},
		{"too few octets", "192.168.1", false},
		{"letters", "abc.def.ghi.jkl", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ipv4Valid(tt.input)
			if got != tt.valid {
				t.Fatalf("ipv4Valid(%q) = %v, want %v", tt.input, got, tt.valid)
			}
		})
	}
}

func TestScanTextAllEntityTypes(t *testing.T) {
	text := `Contact john@example.com or call 555-123-4567.
SSN: 123-45-6789. API key: api_key=Abc1234567890123456.
Card: 4111111111111111. Server: 192.168.1.100.
Born: 1990-01-15. Zip: 90210.`

	findings := ScanText(text, nil)

	found := map[string]bool{}
	for _, f := range findings {
		found[f.EntityType] = true
	}

	expected := []string{"email", "phone", "ssn", "api_key", "credit_card", "ip_address", "date", "zip_code"}
	for _, et := range expected {
		if !found[et] {
			t.Errorf("expected entity type %q to be detected, found types: %v", et, found)
		}
	}
}
