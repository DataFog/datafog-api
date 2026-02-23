package shim

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/datafog/datafog-api/internal/models"
)

type DecisionClient interface {
	Decide(ctx context.Context, req models.DecideRequest) (models.DecideResponse, error)
}

type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Details    string
}

func (e APIError) Error() string {
	return fmt.Sprintf("datafog policy API error: status=%d code=%s message=%s", e.StatusCode, e.Code, e.Message)
}

type HTTPDecisionClient struct {
	DecisionEndpoint string
	APIKey           string
	HTTPClient       *http.Client
}

func NewHTTPDecisionClient(baseURL, apiKey string) *HTTPDecisionClient {
	return &HTTPDecisionClient{
		DecisionEndpoint: buildDecideEndpoint(baseURL),
		APIKey:           strings.TrimSpace(apiKey),
		HTTPClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *HTTPDecisionClient) Decide(ctx context.Context, req models.DecideRequest) (models.DecideResponse, error) {
	var out models.DecideResponse
	if c.HTTPClient == nil {
		return out, fmt.Errorf("http client is not configured")
	}
	if strings.TrimSpace(c.DecisionEndpoint) == "" {
		return out, fmt.Errorf("decision endpoint is required")
	}

	body, err := json.Marshal(req)
	if err != nil {
		return out, fmt.Errorf("marshal decide request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.DecisionEndpoint, bytes.NewReader(body))
	if err != nil {
		return out, fmt.Errorf("create decide request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		httpReq.Header.Set("X-API-Key", c.APIKey)
	}

	resp, err := c.HTTPClient.Do(httpReq)
	if err != nil {
		return out, fmt.Errorf("call decide API: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return out, fmt.Errorf("read decide response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		apiErr := APIError{
			StatusCode: resp.StatusCode,
		}
		var parsed struct {
			Error models.APIError `json:"error"`
		}
		if err := json.Unmarshal(respBody, &parsed); err == nil {
			apiErr.Code = parsed.Error.Code
			apiErr.Message = parsed.Error.Message
			apiErr.Details = parsed.Error.Details
		} else {
			apiErr.Message = strings.TrimSpace(string(respBody))
			if apiErr.Message == "" {
				apiErr.Message = "policy service error"
			}
		}
		return out, apiErr
	}

	if err := json.Unmarshal(respBody, &out); err != nil {
		return out, fmt.Errorf("unmarshal decide response: %w", err)
	}
	return out, nil
}

func buildDecideEndpoint(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		baseURL = "http://localhost:8080"
	}
	if strings.HasSuffix(baseURL, "/v1/decide") {
		return baseURL
	}
	if strings.HasSuffix(baseURL, "/v1") {
		return baseURL + "/decide"
	}
	return strings.TrimRight(baseURL, "/") + "/v1/decide"
}
