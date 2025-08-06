package llm

import (
	"time"
)

// ModelConfig represents an Ollama model configuration
type ModelConfig struct {
	Name    string  `json:"name"`
	Options Options `json:"options"`
}

// Options represents Ollama generation options
type Options struct {
	NumCtx      int     `json:"num_ctx,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	TopK        int     `json:"top_k,omitempty"`
	TopP        float64 `json:"top_p,omitempty"`
	Seed        int64   `json:"seed,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
	NumGPU      *int    `json:"num_gpu,omitempty"`
}

// GenerateRequest represents a request to Ollama's generate endpoint
type GenerateRequest struct {
	Model     string  `json:"model"`
	System    string  `json:"system,omitempty"`
	Prompt    string  `json:"prompt"`
	Options   Options `json:"options,omitempty"`
	KeepAlive string  `json:"keep_alive,omitempty"`
	Stream    bool    `json:"stream,omitempty"`
}

// GenerateResponse represents a response from Ollama's generate endpoint
type GenerateResponse struct {
	Model              string        `json:"model"`
	CreatedAt          time.Time     `json:"created_at"`
	Response           string        `json:"response"`
	Done               bool          `json:"done"`
	DoneReason         string        `json:"done_reason,omitempty"`
	Context            []int         `json:"context,omitempty"`
	TotalDuration      time.Duration `json:"total_duration"`
	LoadDuration       time.Duration `json:"load_duration"`
	PromptEvalCount    int           `json:"prompt_eval_count"`
	PromptEvalDuration time.Duration `json:"prompt_eval_duration"`
	EvalCount          int           `json:"eval_count"`
	EvalDuration       time.Duration `json:"eval_duration"`
}

// CreateDefaultModelConfig creates a ModelConfig with sensible defaults
func CreateDefaultModelConfig(name string) ModelConfig {
	return ModelConfig{
		Name: name,
		Options: Options{
			NumCtx:      4096,
			Temperature: 0.8,
			TopK:        40,
			TopP:        0.9,
			Seed:        0b111001111110011101101110001011011111101100110111111001111111001, // symbology from Python
			NumPredict:  -1,
			NumGPU:      nil,
		},
	}
}

// CreateQwen3ModelConfig creates a ModelConfig for qwen3:14b with expanded context
func CreateQwen3ModelConfig() ModelConfig {
	return ModelConfig{
		Name: "qwen3:14b",
		Options: Options{
			NumCtx:      8000, // 20k token context to match notebook
			Temperature: 0.8,
			TopK:        40,
			TopP:        0.9,
			Seed:        0b111001111110011101101110001011011111101100110111111001111111001,
			NumPredict:  -1,
			NumGPU:      nil,
		},
	}
}
