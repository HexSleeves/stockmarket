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

const geminiBaseURL = "https://generativelanguage.googleapis.com/v1beta/models"

// Gemini implements the Analyzer interface for Google Gemini API
type Gemini struct {
	apiKey string
	model  string
	client *http.Client
}

// NewGemini creates a new Gemini analyzer
func NewGemini(apiKey string, model string) *Gemini {
	if model == "" {
		model = "gemini-pro"
	}
	return &Gemini{
		apiKey: apiKey,
		model:  model,
		client: &http.Client{Timeout: 60 * time.Second},
	}
}

// Name returns the provider name
func (g *Gemini) Name() string {
	return "gemini"
}

// Analyze performs stock analysis using Gemini
func (g *Gemini) Analyze(ctx context.Context, req models.AnalysisRequest) (*models.AnalysisResponse, error) {
	if g.apiKey == "" {
		return nil, ErrNoAPIKey
	}

	prompt := BuildPrompt(req)

	url := fmt.Sprintf("%s/%s:generateContent?key=%s", geminiBaseURL, g.model, g.apiKey)

	requestBody := map[string]interface{}{
		"contents": []map[string]interface{}{
			{
				"parts": []map[string]string{
					{"text": prompt},
				},
			},
		},
		"generationConfig": map[string]interface{}{
			"temperature": 0.3,
			"maxOutputTokens": 1000,
		},
	}

	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return nil, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, err
	}

	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := g.client.Do(httpReq)
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
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	if len(result.Candidates) == 0 || len(result.Candidates[0].Content.Parts) == 0 {
		return nil, ErrAnalysisFailed
	}

	return parseAnalysisResponse(req.Symbol, result.Candidates[0].Content.Parts[0].Text)
}
