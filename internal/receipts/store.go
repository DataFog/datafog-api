package receipts

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/datafog/datafog-api/internal/models"
	"github.com/datafog/datafog-api/internal/policy"
)

const maxReceiptLineBytes = 1024 * 1024

type ReceiptStore struct {
	mu       sync.RWMutex
	filePath string
	receipts map[string]models.Receipt
}

func NewReceiptStore(filePath string) (*ReceiptStore, error) {
	if filePath == "" {
		filePath = "datafog_receipts.jsonl"
	}
	dir := filepath.Dir(filePath)
	if dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
	}

	store := &ReceiptStore{
		filePath: filePath,
		receipts: map[string]models.Receipt{},
	}
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDONLY, 0o644)
	if err != nil {
		return nil, err
	}
	f.Close()
	if err := store.loadExistingReceipts(); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *ReceiptStore) NewReceipt(req models.DecideRequest, decision models.Decision, result policy.DecisionResult, policyMeta models.Policy) models.Receipt {
	return models.Receipt{
		ReceiptID:     newID(),
		Timestamp:     time.Now().UTC(),
		RequestID:     req.RequestID,
		TraceID:       req.TraceID,
		TenantID:      req.TenantID,
		ActorID:       req.ActorID,
		SessionID:     req.SessionID,
		PolicyVersion: policyMeta.PolicyVersion,
		PolicyID:      policyMeta.PolicyID,
		Decision:      decision,
		Action:        req.Action,
		MatchedRules:  result.MatchedRules,
		Findings:      req.Findings,
		TransformPlan: result.TransformPlan,
		Reason:        result.Reason,
	}
}

func (s *ReceiptStore) Save(receipt models.Receipt) (models.Receipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if receipt.ReceiptID == "" {
		receipt.ReceiptID = newID()
	}

	data, err := json.Marshal(receipt)
	if err != nil {
		return models.Receipt{}, err
	}

	f, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return models.Receipt{}, err
	}
	defer f.Close()

	if _, err := f.Write(appendWithLine(data)); err != nil {
		return models.Receipt{}, err
	}
	if err := f.Sync(); err != nil {
		return models.Receipt{}, err
	}

	s.receipts[receipt.ReceiptID] = receipt
	return receipt, nil
}

func (s *ReceiptStore) loadExistingReceipts() error {
	f, err := os.OpenFile(s.filePath, os.O_RDONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64*1024), maxReceiptLineBytes)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var receipt models.Receipt
		if err := json.Unmarshal([]byte(line), &receipt); err != nil {
			return fmt.Errorf("decode existing receipt: %w", err)
		}
		s.receipts[receipt.ReceiptID] = receipt
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	return nil
}

func appendWithLine(data []byte) []byte {
	return append(append(make([]byte, 0, len(data)+1), data...), '\n')
}

func (s *ReceiptStore) Get(id string) (models.Receipt, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.receipts[id]
	return r, ok
}

func newID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(buf)
}
