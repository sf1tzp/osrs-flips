package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/pkoukk/tiktoken-go"
)

// Client handles communication with Ollama API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new Ollama client
func NewClient(baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = "http://10.0.0.4:8000"
	}

	if timeout == 0 {
		timeout = 5 * time.Minute // Reduced default timeout
	}

	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// CheckConnection verifies that Ollama is available
func (c *Client) CheckConnection(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.baseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to Ollama at %s: %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	return nil
}

// Generate sends a generate request to Ollama
func (c *Client) Generate(ctx context.Context, config ModelConfig, systemPrompt, userPrompt string) (*GenerateResponse, error) {
	request := GenerateRequest{
		Model:     config.Name,
		System:    systemPrompt,
		Prompt:    userPrompt,
		Options:   config.Options,
		KeepAlive: "30m",
		Stream:    false, // Disable streaming for simpler response handling
	}

	// Count input tokens accurately
	inputTokens := countTokensForModel(systemPrompt+userPrompt, config.Name)
	log.Printf("Sending generate request to model %s (%d input tokens)", config.Name, inputTokens)

	if inputTokens > config.Options.NumCtx {
		log.Printf("Warning: estimated input tokens (%d) exceeds context size (%d)", inputTokens, config.Options.NumCtx)
	}

	jsonData, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/api/generate", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	startTime := time.Now()
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(body))
	}

	var response GenerateResponse

	// For non-streaming, Ollama returns a single JSON object
	// For streaming, it returns multiple JSON objects separated by newlines
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Try to decode as a single JSON object first
	if err := json.Unmarshal(body, &response); err != nil {
		// If that fails, try to handle as streaming response
		lines := strings.Split(string(body), "\n")
		var fullResponse strings.Builder

		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}

			var streamResponse GenerateResponse
			if err := json.Unmarshal([]byte(line), &streamResponse); err != nil {
				continue // Skip invalid lines
			}

			// Accumulate the response text
			fullResponse.WriteString(streamResponse.Response)

			// Use the last response for metadata
			if streamResponse.Done {
				response = streamResponse
				response.Response = fullResponse.String()
				break
			}
		}

		// If we didn't get a proper response, return error
		if response.Response == "" {
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}
	}

	// Calculate metrics
	duration := time.Since(startTime)
	outputTokens := countTokensForModel(response.Response, config.Name)
	tokensPerSecond := float64(outputTokens) / duration.Seconds()

	log.Printf("Received generate response from %s: duration=%.2fs, output_tokens=%d, tokens_per_second=%.2f",
		config.Name, duration.Seconds(), outputTokens, tokensPerSecond)

	return &response, nil
}

// RemoveThinkingTags removes <think>...</think> blocks from LLM responses
func RemoveThinkingTags(content string) string {
	if content == "" {
		return ""
	}

	// Remove <think>...</think> blocks and any content before them
	re := regexp.MustCompile(`(?i)<think>[\s\S]*?</think>\s*`)
	cleaned := re.ReplaceAllString(content, "")

	// Trim any remaining whitespace
	cleaned = strings.TrimSpace(cleaned)

	return cleaned
}

// countTokensForModel provides model-specific token counting
func countTokensForModel(content string, modelName string) int {

	// TODO: tiktoken has a `tiktoken.EncodingForModel`

	encoding, err := tiktoken.EncodingForModel("gpt2")

	// var encodingName string
	//
	// // Choose encoding based on model
	// switch {
	// case strings.Contains(strings.ToLower(modelName), "qwen"):
	// 	// Qwen models typically use cl100k_base or similar
	// 	encodingName = "cl100k_base"
	// case strings.Contains(strings.ToLower(modelName), "gemma"):
	// 	// Gemma models use cl100k_base encoding
	// 	encodingName = "cl100k_base"
	// case strings.Contains(strings.ToLower(modelName), "gpt-4"):
	// 	encodingName = "cl100k_base"
	// case strings.Contains(strings.ToLower(modelName), "gpt-3.5"):
	// 	encodingName = "cl100k_base"
	// default:
	// 	// Default to cl100k_base which works for most modern models
	// 	encodingName = "cl100k_base"
	// }
	//
	// encoding, err := tiktoken.GetEncoding(encodingName)
	if err != nil {
		// Fallback to estimation if tiktoken fails
		log.Printf("Warning: tiktoken encoding failed for %s, using estimation: %v", "gpt2", err)
		return fallbackTokenCount(content)
	}

	tokens := encoding.Encode(content, nil, nil)
	return len(tokens)
}

// fallbackTokenCount provides estimation when tiktoken is unavailable
func fallbackTokenCount(content string) int {
	// More accurate approximation than simple /4
	words := len(strings.Fields(content))
	chars := len(content)

	// Estimate based on both word count and character count
	wordBasedTokens := float64(words) * 0.75
	charBasedTokens := float64(chars) * 0.2

	// Use the average of both methods for better accuracy
	estimatedTokens := int((wordBasedTokens + charBasedTokens) / 2)

	// Ensure we don't return 0 for non-empty content
	if estimatedTokens == 0 && len(content) > 0 {
		return 1
	}

	return estimatedTokens
}

// GenerateWithRetry generates with exponential backoff retry logic
func (c *Client) GenerateWithRetry(ctx context.Context, config ModelConfig, systemPrompt, userPrompt string, maxRetries int) (*GenerateResponse, error) {
	var lastErr error
	backoff := 1 * time.Second

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff = time.Duration(math.Min(60, float64(backoff)*2))
			}
			log.Printf("Retrying generate request (attempt %d/%d)", attempt+1, maxRetries+1)
		}

		response, err := c.Generate(ctx, config, systemPrompt, userPrompt)
		if err == nil {
			return response, nil
		}

		lastErr = err
		log.Printf("Generate attempt %d failed: %v", attempt+1, err)
	}

	return nil, fmt.Errorf("generate failed after %d attempts: %w", maxRetries+1, lastErr)
}

// GetGenerateResponse provides a simplified interface matching the Python notebook pattern
// Equivalent to Python's get_generate_response(model_config, system_prompt, user_prompt)
func GetGenerateResponse(ctx context.Context, client *Client, modelConfig ModelConfig, systemPrompt, userPrompt string) (*GenerateResponse, error) {
	// Use Generate directly for simple interface
	response, err := client.Generate(ctx, modelConfig, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("generate request failed: %w", err)
	}

	return response, nil
}
