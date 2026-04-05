package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/generative-ai-go/genai"
	"google.golang.org/api/option"
)

type aiProvider string

const (
	aiProviderAuto       aiProvider = "auto"
	aiProviderGemini     aiProvider = "gemini"
	aiProviderOpenRouter aiProvider = "openrouter"
	aiProviderOpenAI     aiProvider = "openai"
)

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func normalizeAIProvider(raw string) aiProvider {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(aiProviderAuto):
		return aiProviderAuto
	case string(aiProviderGemini):
		return aiProviderGemini
	case string(aiProviderOpenRouter), "openrouterai", "openrouter-api":
		return aiProviderOpenRouter
	case string(aiProviderOpenAI), "chatgpt":
		return aiProviderOpenAI
	default:
		return aiProviderAuto
	}
}

func resolveAIProvider(preferred string) aiProvider {
	provider := normalizeAIProvider(preferred)
	if provider != aiProviderAuto {
		return provider
	}

	if firstNonEmpty(os.Getenv("OPENROUTER_API_KEY"), os.Getenv("OPEN_ROUTER_API_KEY")) != "" {
		return aiProviderOpenRouter
	}
	if firstNonEmpty(os.Getenv("GEMINI_API_KEY")) != "" {
		return aiProviderGemini
	}
	if firstNonEmpty(os.Getenv("OPENAI_API_KEY"), os.Getenv("CHATGPT_API_KEY")) != "" {
		return aiProviderOpenAI
	}

	return aiProviderGemini
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func providerModelName(provider aiProvider) string {
	switch provider {
	case aiProviderOpenRouter:
		return firstNonEmpty(os.Getenv("OPENROUTER_MODEL"), "openai/gpt-4o-mini")
	case aiProviderOpenAI:
		return firstNonEmpty(os.Getenv("OPENAI_MODEL"), "gpt-4o-mini")
	default:
		return firstNonEmpty(os.Getenv("GEMINI_MODEL"), "gemini-2.5-flash-lite")
	}
}

func generateAIText(ctx context.Context, provider aiProvider, systemPrompt, userPrompt string) (string, error) {
	provider = resolveAIProvider(string(provider))
	switch provider {
	case aiProviderGemini:
		return generateWithGemini(ctx, systemPrompt, userPrompt)
	case aiProviderOpenRouter, aiProviderOpenAI:
		return generateWithChatCompletions(ctx, provider, systemPrompt, userPrompt)
	default:
		return generateWithGemini(ctx, systemPrompt, userPrompt)
	}
}

func generateWithGemini(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	apiKey := firstNonEmpty(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" {
		return "", fmt.Errorf("missing GEMINI_API_KEY")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return "", fmt.Errorf("failed to create Gemini client: %w", err)
	}
	defer client.Close()

	model := client.GenerativeModel(providerModelName(aiProviderGemini))
	resp, err := model.GenerateContent(ctx, genai.Text(systemPrompt+"\n\n"+userPrompt))
	if err != nil {
		return "", fmt.Errorf("failed to generate content: %w", err)
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response from Gemini")
	}

	var builder strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		builder.WriteString(strings.TrimSpace(fmt.Sprint(part)))
	}
	return strings.TrimSpace(builder.String()), nil
}

func generateWithChatCompletions(ctx context.Context, provider aiProvider, systemPrompt, userPrompt string) (string, error) {
	apiKey := ""
	endpoint := ""
	modelName := providerModelName(provider)

	switch provider {
	case aiProviderOpenRouter:
		apiKey = firstNonEmpty(os.Getenv("OPENROUTER_API_KEY"), os.Getenv("OPEN_ROUTER_API_KEY"))
		endpoint = "https://openrouter.ai/api/v1/chat/completions"
	case aiProviderOpenAI:
		apiKey = firstNonEmpty(os.Getenv("OPENAI_API_KEY"), os.Getenv("CHATGPT_API_KEY"))
		endpoint = "https://api.openai.com/v1/chat/completions"
	default:
		return "", fmt.Errorf("unsupported chat provider: %s", provider)
	}

	if apiKey == "" {
		return "", fmt.Errorf("missing API key for provider %s", provider)
	}

	requestBody := map[string]interface{}{
		"model": modelName,
		"messages": []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		"temperature": 0.2,
	}

	body, err := json.Marshal(requestBody)
	if err != nil {
		return "", fmt.Errorf("failed to encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	if provider == aiProviderOpenRouter {
		req.Header.Set("HTTP-Referer", "http://localhost")
		req.Header.Set("X-Title", "Smart Retail")
	}

	httpClient := &http.Client{Timeout: 60 * time.Second}
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to call %s: %w", provider, err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read AI response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("%s request failed: %s", provider, strings.TrimSpace(string(responseBody)))
	}

	var payload struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(responseBody, &payload); err != nil {
		return "", fmt.Errorf("failed to parse AI response: %w", err)
	}
	if len(payload.Choices) == 0 {
		return "", fmt.Errorf("no choices returned by %s", provider)
	}

	content := strings.TrimSpace(payload.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("empty response from %s", provider)
	}

	return content, nil
}

func stripHTMLTags(input string) string {
	var builder strings.Builder
	inTag := false
	for _, r := range input {
		switch r {
		case '<':
			inTag = true
		case '>':
			inTag = false
		case '\n':
			if !inTag {
				builder.WriteRune(' ')
			}
		default:
			if !inTag {
				builder.WriteRune(r)
			}
		}
	}
	cleaned := strings.ReplaceAll(builder.String(), "&nbsp;", " ")
	cleaned = strings.ReplaceAll(cleaned, "&amp;", "&")
	cleaned = strings.ReplaceAll(cleaned, "&lt;", "<")
	cleaned = strings.ReplaceAll(cleaned, "&gt;", ">")
	return strings.Join(strings.Fields(cleaned), " ")
}
