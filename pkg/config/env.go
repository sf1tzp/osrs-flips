package config

import (
	"fmt"
	"os"
)

// LoadConfigForMain loads configuration for main program (without Discord validation)
func LoadConfigForMain(configPath string) (*Config, error) {
	// Start with minimal defaults (let YAML override)
	config := &Config{
		LLM: LLMConfig{
			BaseURL: "http://localhost:11434",
			Model:   "qwen3:14b",
			// Timeout will be set from YAML or default in GetTimeout()
		},
		OSRS: OSRSConfig{
			UserAgent:          "",
			MaxConcurrentCalls: 3,
			RateLimitDelayMs:   500,
			VolumeDataMaxItems: 50,
		},
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
	}

	// Load from YAML file if it exists
	if configPath != "" {
		if err := loadYAMLFile(configPath, config); err != nil {
			if !os.IsNotExist(err) {
				return nil, fmt.Errorf("failed to load config file %s: %w", configPath, err)
			}
		}
	}

	// Override with environment variables
	loadEnvironmentVariables(config)

	// Validate only what's needed for main program
	if err := validateMainConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// validateMainConfig validates config for main program (no Discord requirement)
func validateMainConfig(config *Config) error {
	if config.OSRS.UserAgent == "" {
		return fmt.Errorf("a user agent string must be configured")
	}

	if len(config.Jobs) == 0 {
		return fmt.Errorf("at least one job must be configured")
	}

	// Validate that at least one job is enabled
	hasEnabledJob := false
	for _, job := range config.Jobs {
		if job.Enabled {
			hasEnabledJob = true
			break
		}
	}

	if !hasEnabledJob {
		return fmt.Errorf("at least one job must be enabled")
	}

	return nil
}
