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
				t.Fatalf("expected %d findings, got %d", len(vector.Findings), len(got))
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
				End:        19,
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
