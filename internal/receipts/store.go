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
const defaultReceiptFileMode = 0o600
const defaultReceiptDirMode = 0o750

type ReceiptStore struct {
	mu          sync.RWMutex
	filePath    string
	receipts    map[string]models.Receipt
	maxEntries  int
	entryCount  int
}

// MaxEntries sets the maximum number of receipts before rotation.
// 0 means no limit (default).
func MaxEntries(n int) func(*ReceiptStore) {
	return func(s *ReceiptStore) {
		s.maxEntries = n
	}
}

func NewReceiptStore(filePath string, opts ...func(*ReceiptStore)) (*ReceiptStore, error) {
	if filePath == "" {
		filePath = "datafog_receipts.jsonl"
	}
	filePath = strings.TrimSpace(filePath)
	if strings.ContainsRune(filePath, 0) {
		return nil, fmt.Errorf("invalid receipt path")
	}
	dir := filepath.Dir(filePath)
	if dir != "." {
		if err := os.MkdirAll(dir, defaultReceiptDirMode); err != nil {
			return nil, err
		}
	}

	store := &ReceiptStore{
		filePath: filePath,
		receipts: map[string]models.Receipt{},
	}
	for _, opt := range opts {
		opt(store)
	}
	f, err := os.OpenFile(filePath, os.O_CREATE|os.O_RDONLY, defaultReceiptFileMode) // #nosec G304 -- receipt path is validated from startup configuration.
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

	// Rotate if we've hit the max
	if s.maxEntries > 0 && s.entryCount >= s.maxEntries {
		if err := s.rotateLocked(); err != nil {
			return models.Receipt{}, fmt.Errorf("receipt rotation failed: %w", err)
		}
	}

	data, err := json.Marshal(receipt)
	if err != nil {
		return models.Receipt{}, err
	}

	f, err := os.OpenFile(s.filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, defaultReceiptFileMode) // #nosec G304 -- receipt path is validated from startup configuration.
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
	s.entryCount++
	return receipt, nil
}

// rotateLocked archives the current receipts file and starts fresh.
// Must be called with s.mu held.
func (s *ReceiptStore) rotateLocked() error {
	archivePath := s.filePath + "." + time.Now().UTC().Format("20060102T150405Z")
	if err := os.Rename(s.filePath, archivePath); err != nil && !os.IsNotExist(err) {
		return err
	}
	s.receipts = map[string]models.Receipt{}
	s.entryCount = 0
	return nil
}

// Count returns the number of receipts in memory.
func (s *ReceiptStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.receipts)
}

func (s *ReceiptStore) loadExistingReceipts() error {
	f, err := os.OpenFile(s.filePath, os.O_RDONLY, defaultReceiptFileMode) // #nosec G304 -- receipt path is validated from startup configuration.
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
		s.entryCount++
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
