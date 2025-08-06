package config

import (
	"testing"
	"time"
)

func TestJobModelConfigOverride(t *testing.T) {
	// Create a test configuration
	globalLLM := &LLMConfig{
		Model:   "qwen3:14b",
		Timeout: "10m",
	}

	// Test job with model overrides
	job := JobConfig{
		Name: "Test Job",
		Model: &JobModelConfig{
			Name:        stringPtr("qwen3:4b"),
			NumCtx:      intPtr(24000),
			Temperature: float64Ptr(0.7),
			Timeout:     stringPtr("5m"),
		},
	}

	// Test model config generation
	modelConfig := job.GetJobModelConfig(globalLLM)
	if modelConfig.Name != "qwen3:4b" {
		t.Errorf("Expected model name 'qwen3:4b', got '%s'", modelConfig.Name)
	}
	if modelConfig.Options.NumCtx != 24000 {
		t.Errorf("Expected num_ctx 24000, got %d", modelConfig.Options.NumCtx)
	}
	if modelConfig.Options.Temperature != 0.7 {
		t.Errorf("Expected temperature 0.7, got %f", modelConfig.Options.Temperature)
	}

	// Test timeout override
	timeout := job.GetJobTimeout(globalLLM)
	expected := 5 * time.Minute
	if timeout != expected {
		t.Errorf("Expected timeout %v, got %v", expected, timeout)
	}
}

func TestJobWithoutModelOverride(t *testing.T) {
	globalLLM := &LLMConfig{
		Model:   "qwen3:14b",
		Timeout: "10m",
	}

	// Job without model config should use global config
	job := JobConfig{
		Name: "Test Job Without Override",
	}

	modelConfig := job.GetJobModelConfig(globalLLM)
	if modelConfig.Name != "qwen3:14b" {
		t.Errorf("Expected model name 'qwen3:14b', got '%s'", modelConfig.Name)
	}

	// Should use global timeout
	timeout := job.GetJobTimeout(globalLLM)
	expected := 10 * time.Minute
	if timeout != expected {
		t.Errorf("Expected timeout %v, got %v", expected, timeout)
	}
}

func TestGetJobByName(t *testing.T) {
	config := &Config{
		Jobs: []JobConfig{
			{Name: "Job 1"},
			{Name: "Job 2"},
		},
	}

	// Test finding existing job
	job := config.GetJobByName("Job 1")
	if job == nil {
		t.Error("Expected to find 'Job 1', got nil")
	} else if job.Name != "Job 1" {
		t.Errorf("Expected job name 'Job 1', got '%s'", job.Name)
	}

	// Test non-existing job
	job = config.GetJobByName("Non-existent")
	if job != nil {
		t.Error("Expected nil for non-existent job, got a job")
	}
}

// Helper functions for creating pointers
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}
