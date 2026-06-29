package services

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ClaudeClient handles all Claude API interactions.
type ClaudeClient struct {
	apiKey  string
	model   string
	client  *http.Client
	baseURL string
}

func NewClaudeClient() *ClaudeClient {
	return &ClaudeClient{
		apiKey:  os.Getenv("ANTHROPIC_API_KEY"),
		model:   os.Getenv("ANTHROPIC_MODEL"),
		client:  &http.Client{Timeout: 120 * time.Second},
		baseURL: "https://api.anthropic.com/v1",
	}
}

// ParseCV sends a PDF/DOCX file to Claude and returns structured resume data.
// fileBytes: raw file bytes
// mimeType: "application/pdf" or "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
func (c *ClaudeClient) ParseCV(ctx context.Context, fileBytes []byte, mimeType string) (*ResumeData, error) {
	encoded := base64.StdEncoding.EncodeToString(fileBytes)

	// The system prompt (including the full extraction schema) is identical
	// across every request, so it's marked for prompt caching — repeat calls
	// skip reprocessing those ~1k+ tokens, which cuts both latency and cost.
	// Only the document itself and a short instruction vary per request.
	requestBody := map[string]any{
		"model":      c.model,
		"max_tokens": 8192,
		"system": []map[string]any{
			{
				"type":          "text",
				"text":          systemPrompt() + "\n\n" + buildExtractionPrompt(),
				"cache_control": map[string]string{"type": "ephemeral"},
			},
		},
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "document",
						"source": map[string]any{
							"type":       "base64",
							"media_type": mimeType,
							"data":       encoded,
						},
					},
					{
						"type": "text",
						"text": "Extract this CV into the JSON schema from your instructions. Return ONLY the JSON object — no markdown, no commentary.",
					},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		c.baseURL+"/messages", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("anthropic-beta", "pdfs-2024-09-25")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("api request: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("claude api error %d: %s", resp.StatusCode, string(respBytes))
	}

	// Parse Claude's response envelope
	var claudeResp struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(respBytes, &claudeResp); err != nil {
		return nil, fmt.Errorf("unmarshal claude response: %w", err)
	}

	if len(claudeResp.Content) == 0 {
		return nil, fmt.Errorf("empty response from claude")
	}

	// Extract the JSON from the text response. Some models wrap the JSON in
	// a markdown code fence despite instructions not to — strip it if present.
	rawJSON := extractJSON(claudeResp.Content[0].Text)

	var resume ResumeData
	if err := json.Unmarshal([]byte(rawJSON), &resume); err != nil {
		return nil, fmt.Errorf("unmarshal resume JSON: %w\nraw: %s", err, rawJSON)
	}

	return &resume, nil
}

func extractJSON(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```json")
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}

func systemPrompt() string {
	return `You are a precise CV/resume parser. Your job is to extract structured data from any CV or resume document, regardless of format, language, layout, or profession. You always return valid JSON matching the exact schema requested. You never add commentary, explanations, or markdown formatting. You return only the JSON object.`
}

func buildExtractionPrompt() string {
	return `Extract all information from this CV/resume and return it as a single JSON object matching this exact schema. Include every section that exists in the document, even if not listed below. For missing fields, use null or empty arrays — never omit keys.

Return ONLY valid JSON, no explanation, no markdown, no code blocks.

{
  "full_name": "string",
  "headline": "string — current job title or professional tagline",
  "summary": "string — professional summary or bio, full text",
  "email": "string or null",
  "phone": "string or null",
  "location": "string or null — city, country",
  "linkedin_url": "string or null",
  "github_url": "string or null",
  "website_url": "string or null",
  "career_type": "one of: developer, designer, creative, corporate, academic, healthcare, education, hospitality, legal, finance, marketing, general",
  "suggested_theme": "one of: professional, creative, minimal",
  "experience": [
    {
      "title": "string — job title",
      "company": "string",
      "location": "string or null",
      "start_date": "string e.g. Jan 2020",
      "end_date": "string e.g. Dec 2023 or Present",
      "description": "string or null — full description paragraph if present",
      "bullets": ["string", "string"],
      "url": "string or null"
    }
  ],
  "education": [
    {
      "degree": "string e.g. BSc Computer Science",
      "institution": "string",
      "location": "string or null",
      "start_date": "string or null",
      "end_date": "string e.g. 2022",
      "description": "string or null",
      "grade": "string or null"
    }
  ],
  "skills": [
    {
      "category": "string e.g. Programming Languages, Tools, Soft Skills",
      "items": ["string", "string"]
    }
  ],
  "projects": [
    {
      "name": "string",
      "description": "string",
      "url": "string or null",
      "start_date": "string or null",
      "end_date": "string or null",
      "technologies": ["string"],
      "bullets": ["string"]
    }
  ],
  "certifications": [
    {
      "name": "string",
      "issuer": "string or null",
      "date": "string or null",
      "url": "string or null",
      "credential_id": "string or null"
    }
  ],
  "awards": [
    {
      "title": "string",
      "issuer": "string or null",
      "date": "string or null",
      "description": "string or null"
    }
  ],
  "publications": [
    {
      "title": "string",
      "publisher": "string or null",
      "date": "string or null",
      "url": "string or null",
      "description": "string or null"
    }
  ],
  "volunteer": [
    {
      "role": "string",
      "organization": "string",
      "start_date": "string or null",
      "end_date": "string or null",
      "description": "string or null"
    }
  ],
  "languages": [
    {
      "language": "string",
      "proficiency": "string e.g. Native, Fluent, Intermediate"
    }
  ],
  "interests": ["string"],
  "custom_sections": [
    {
      "title": "string — the section heading as it appears in the CV",
      "items": [
        {
          "title": "string or null",
          "description": "string or null"
        }
      ]
    }
  ]
}`
}
