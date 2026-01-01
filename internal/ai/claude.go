package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"stockmarket/internal/models"
)

const claudeBaseURL = "https://api.anthropic.com/v1/messages"

// Claude implements the Analyzer interface for Anthropic Claude API
type Claude struct {
	apiKey string
	model  string
	client *http.Client
}

// NewClaude creates a new Claude analyzer
func NewClaude(apiKey string, model string) *Claude {
	if model == "" {
		model = "claude-sonnet-4-20250514"
	}
	return &Claude{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Name returns the provider name
func (c *Claude) Name() string {
	return "claude"
}

// Analyze performs stock analysis using Claude
func (c *Claude) Analyze(ctx context.Context, req models.AnalysisRequest) (*models.AnalysisResponse, error) {
	if c.apiKey == "" {
		return nil, ErrNoAPIKey
	}

	prompt := BuildPrompt(req)

	requestBody := map[string]interface{}{
		"model":      c.model,
		"max_tokens": 1000,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", claudeBaseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("x-api-key", c.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.client.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		var errResp struct {
			Error struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		json.NewDecoder(resp.Body).Decode(&errResp)
		return nil, fmt.Errorf("%w: %s", ErrAnalysisFailed, errResp.Error.Message)
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Content) == 0 {
		return nil, ErrAnalysisFailed
	}

	return parseAnalysisResponse(req.Symbol, result.Content[0].Text)
}
