package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"stockmarket/internal/models"
)

const openAIBaseURL = "https://api.openai.com/v1/chat/completions"

// OpenAI implements the Analyzer interface for OpenAI API
type OpenAI struct {
	apiKey string
	model  string
	client *http.Client
}

// NewOpenAI creates a new OpenAI analyzer
func NewOpenAI(apiKey string, model string) *OpenAI {
	if model == "" {
		model = "gpt-4o"
	}
	return &OpenAI{
		apiKey: apiKey,
		model:  model,
		client: sharedHTTPClient,
	}
}

// Name returns the provider name
func (o *OpenAI) Name() string {
	return "openai"
}

// Analyze performs stock analysis using OpenAI
func (o *OpenAI) Analyze(ctx context.Context, req models.AnalysisRequest) (*models.AnalysisResponse, error) {
	if o.apiKey == "" {
		return nil, ErrNoAPIKey
	}

	prompt := BuildPrompt(req)

	requestBody := map[string]interface{}{
		"model": o.model,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"temperature": 0.3,
		"max_tokens":  1000,
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", openAIBaseURL, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+o.apiKey)

	resp, err := o.client.Do(httpReq)
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
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Choices) == 0 {
		return nil, ErrAnalysisFailed
	}

	return parseAnalysisResponse(req.Symbol, result.Choices[0].Message.Content)
}

// parseAnalysisResponse parses the AI response into an AnalysisResponse
func parseAnalysisResponse(symbol string, content string) (*models.AnalysisResponse, error) {
	// Try to extract JSON from the response
	content = strings.TrimSpace(content)

	// Handle markdown code blocks
	if strings.HasPrefix(content, "```json") {
		content = strings.TrimPrefix(content, "```json")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	} else if strings.HasPrefix(content, "```") {
		content = strings.TrimPrefix(content, "```")
		content = strings.TrimSuffix(content, "```")
		content = strings.TrimSpace(content)
	}

	var response struct {
		Action       string              `json:"action"`
		Confidence   float64             `json:"confidence"`
		Reasoning    string              `json:"reasoning"`
		PriceTargets models.PriceTargets `json:"price_targets"`
		Risks        []string            `json:"risks"`
		Timeframe    string              `json:"timeframe"`
	}

	if err := json.Unmarshal([]byte(content), &response); err != nil {
		return nil, fmt.Errorf("%w: failed to parse response: %v", ErrAnalysisFailed, err)
	}

	return &models.AnalysisResponse{
		Symbol:       symbol,
		Action:       response.Action,
		Confidence:   response.Confidence,
		Reasoning:    response.Reasoning,
		PriceTargets: response.PriceTargets,
		Risks:        response.Risks,
		Timeframe:    response.Timeframe,
		GeneratedAt:  time.Now(),
	}, nil
}
