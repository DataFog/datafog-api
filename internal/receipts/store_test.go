package receipts

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/datafog/datafog-api/internal/models"
	"github.com/datafog/datafog-api/internal/policy"
)

func TestReceiptStoreSaveAndGet(t *testing.T) {
	path := t.TempDir() + "/receipts.jsonl"
	store, err := NewReceiptStore(path)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}

	req := models.DecideRequest{RequestID: "r1", Action: models.ActionMeta{Type: "file.read", Resource: "x"}}
	result := policy.DecisionResult{Decision: models.DecisionAllow, MatchedRules: []string{"allow-1"}}
	policyMeta := models.Policy{PolicyID: "m", PolicyVersion: "v1"}
	receipt := store.NewReceipt(req, models.DecisionAllow, result, policyMeta)
	saved, err := store.Save(receipt)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}
	if saved.ReceiptID == "" {
		t.Fatalf("receipt id empty")
	}
	got, ok := store.Get(saved.ReceiptID)
	if !ok {
		t.Fatalf("receipt not found")
	}
	if got.Decision != models.DecisionAllow {
		t.Fatalf("expected allow receipt")
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("receipt file missing: %v", err)
	}
}

func TestReceiptStoreLoadsExistingReceipts(t *testing.T) {
	path := t.TempDir() + "/receipts.jsonl"
	existing := models.Receipt{
		ReceiptID:     "receipt-seeded",
		PolicyID:      "policy-1",
		PolicyVersion: "v1",
		RequestID:     "r1",
		Decision:      models.DecisionDeny,
		MatchedRules:  []string{"seed"},
	}
	data, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("seed file write failed: %v", err)
	}

	store, err := NewReceiptStore(path)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	got, ok := store.Get("receipt-seeded")
	if !ok {
		t.Fatalf("expected to load existing receipt")
	}
	if got.Decision != models.DecisionDeny {
		t.Fatalf("unexpected decision: %s", got.Decision)
	}
}

func TestReceiptStoreRejectsCorruptReceiptLine(t *testing.T) {
	path := t.TempDir() + "/receipts.jsonl"
	if err := os.WriteFile(path, []byte("{\n"), 0o644); err != nil {
		t.Fatalf("seed file write failed: %v", err)
	}

	if _, err := NewReceiptStore(path); err == nil {
		t.Fatalf("expected receipt load failure on corrupt line")
	}
}

func TestReceiptStoreLoadsLargeReceiptLine(t *testing.T) {
	path := t.TempDir() + "/receipts.jsonl"
	existing := models.Receipt{
		ReceiptID: "receipt-large",
		PolicyID:  "policy-1",
		Findings: []models.ScanFinding{
			{
				EntityType: "email",
				Value:      strings.Repeat("x", 600*1024),
				Start:      0,
				End:        600 * 1024,
				Confidence: 0.9,
			},
		},
		PolicyVersion: "v1",
		RequestID:     "r1",
		Decision:      models.DecisionAllow,
		MatchedRules:  []string{"seed"},
	}
	data, err := json.Marshal(existing)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		t.Fatalf("seed file write failed: %v", err)
	}

	store, err := NewReceiptStore(path)
	if err != nil {
		t.Fatalf("new store failed: %v", err)
	}
	got, ok := store.Get("receipt-large")
	if !ok {
		t.Fatalf("expected to load large receipt")
	}
	if got.ReceiptID != "receipt-large" {
		t.Fatalf("expected loaded receipt id receipt-large, got %q", got.ReceiptID)
	}
}
