package shim

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/datafog/datafog-api/internal/models"
)

func TestBuildDecideEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		expected string
	}{
		{
			name:     "root path",
			base:     "http://localhost:8080",
			expected: "http://localhost:8080/v1/decide",
		},
		{
			name:     "v1 path",
			base:     "http://localhost:8080/v1",
			expected: "http://localhost:8080/v1/decide",
		},
		{
			name:     "full decide path",
			base:     "http://localhost:8080/v1/decide",
			expected: "http://localhost:8080/v1/decide",
		},
	}
	for _, tc := range tests {
		got := buildDecideEndpoint(tc.base)
		if got != tc.expected {
			t.Fatalf("expected %s, got %s", tc.expected, got)
		}
	}
}

func TestHTTPDecisionClientPostsToDecide(t *testing.T) {
	var gotAction models.ActionMeta
	var gotToken string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/decide" {
			t.Fatalf("expected path /v1/decide, got %s", r.URL.Path)
		}
		gotToken = r.Header.Get("X-API-Key")
		var req models.DecideRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request failed: %v", err)
		}
		gotAction = req.Action
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(models.DecideResponse{
			Decision:     models.DecisionAllow,
			ReceiptID:    "r1",
			MatchedRules: []string{"allow-test"},
		})
	}))
	defer server.Close()

	client := NewHTTPDecisionClient(server.URL, "token-1")
	got, err := client.Decide(context.Background(), models.DecideRequest{
		Action: models.ActionMeta{
			Type:      "file.read",
			Resource:  "notes.txt",
			Sensitive: true,
		},
	})
	if err != nil {
		t.Fatalf("decide failed: %v", err)
	}
	if got.Decision != models.DecisionAllow {
		t.Fatalf("expected allow, got %s", got.Decision)
	}
	if gotToken != "token-1" {
		t.Fatalf("expected api token header, got %q", gotToken)
	}
	if gotAction.Type != "file.read" {
		t.Fatalf("expected action type file.read, got %q", gotAction.Type)
	}
}
