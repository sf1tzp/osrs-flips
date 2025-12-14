package config

import (
	"fmt"
	"io"
	"os"
	"time"

	"osrs-flipping/pkg/llm"

	"gopkg.in/yaml.v3"
)

// Config holds the complete application configuration
type Config struct {
	Discord   DiscordConfig    `yaml:"discord"`
	LLM       LLMConfig        `yaml:"llm"`
	OSRS      OSRSConfig       `yaml:"osrs"`
	Logging   LoggingConfig    `yaml:"logging"`
	Jobs      []JobConfig      `yaml:"jobs"`
	Schedules []ScheduleConfig `yaml:"schedules,omitempty"`
}

// DiscordConfig holds Discord bot configuration
type DiscordConfig struct {
	Token     string `yaml:"token" env:"DISCORD_TOKEN"`
	ChannelID string `yaml:"channel_id" env:"DISCORD_CHANNEL_ID"`
	GuildID   string `yaml:"guild_id,omitempty" env:"DISCORD_GUILD_ID"`
}

// LLMConfig holds LLM configuration
type LLMConfig struct {
	BaseURL string `yaml:"base_url" env:"LLM_BASE_URL"`
	Model   string `yaml:"model" env:"LLM_MODEL"`
	Timeout string `yaml:"timeout" env:"LLM_TIMEOUT"`
	NumCtx  int    `yaml:"num_ctx" env:"LLM_NUM_CTX"`
}

// OSRSConfig holds OSRS API configuration
type OSRSConfig struct {
	UserAgent          string `yaml:"user_agent"`
	MaxConcurrentCalls int    `yaml:"max_concurrent_calls"`
	RateLimitDelayMs   int    `yaml:"rate_limit_delay_ms"`
	VolumeDataMaxItems int    `yaml:"volume_data_max_items"`
}

// LoggingConfig holds logging configuration
type LoggingConfig struct {
	Level  string `yaml:"level" env:"LOG_LEVEL"`
	Format string `yaml:"format" env:"LOG_FORMAT"`
}

// JobConfig represents a trading analysis job
type JobConfig struct {
	Name        string          `yaml:"name"`
	Description string          `yaml:"description,omitempty"`
	Filters     FilterConfig    `yaml:"filters"`
	Output      OutputConfig    `yaml:"output,omitempty"`
	Model       *JobModelConfig `yaml:"model,omitempty"`
	Enabled     bool            `yaml:"enabled"`
}

// JobModelConfig represents job-specific model configuration overrides
type JobModelConfig struct {
	Name        *string  `yaml:"name,omitempty"`
	NumCtx      *int     `yaml:"num_ctx,omitempty"`
	Temperature *float64 `yaml:"temperature,omitempty"`
	TopK        *int     `yaml:"top_k,omitempty"`
	TopP        *float64 `yaml:"top_p,omitempty"`
	Seed        *int64   `yaml:"seed,omitempty"`
	NumPredict  *int     `yaml:"num_predict,omitempty"`
	NumGPU      *int     `yaml:"num_gpu,omitempty"`
	Timeout     *string  `yaml:"timeout,omitempty"`
}

// FilterConfig holds all possible filter parameters
type FilterConfig struct {
	// Margin filters
	MarginMin    *int     `yaml:"margin_gp_min,omitempty"`
	MarginPctMin *float64 `yaml:"margin_pct_min,omitempty"`

	// Price filters
	MaxInstaSellPrice1h *int `yaml:"max_insta_sell_price_1h,omitempty"`
	MaxInstaBuyPrice1h  *int `yaml:"max_insta_buy_price_1h,omitempty"`

	// Volume filters
	InstaBuyVolume1hMin  *float64 `yaml:"insta_buy_volume_1h_min,omitempty"`
	InstaSellVolume1hMin *float64 `yaml:"insta_sell_volume_1h_min,omitempty"`
	Volume20mMin         *int     `yaml:"volume_20m_min,omitempty"`
	Volume1hMin          *int     `yaml:"volume_1h_min,omitempty"`
	Volume24hMin         *int     `yaml:"volume_24h_min,omitempty"`
	InstaSellPriceMin    *int     `yaml:"insta_sell_price_min,omitempty"`
	InstaSellPriceMax    *int     `yaml:"insta_sell_price_max,omitempty"`

	// Buy limit filters
	BuyLimitMin *int `yaml:"buy_limit_min,omitempty"`
	BuyLimitMax *int `yaml:"buy_limit_max,omitempty"`

	// Freshness filters
	MaxHoursSinceUpdate *float64 `yaml:"max_hours_since_update,omitempty"`

	// Sorting and limiting
	SortByAfterPrice  string `yaml:"sort_by_after_price,omitempty"`
	SortByAfterVolume string `yaml:"sort_by_after_volume,omitempty"`
	SortDesc          *bool  `yaml:"sort_desc,omitempty"`
	Limit             *int   `yaml:"limit,omitempty"`
}

// OutputConfig controls output formatting
type OutputConfig struct {
	MaxItems int `yaml:"max_items"`
}

// ScheduleConfig defines when jobs should run
type ScheduleConfig struct {
	JobName  string `yaml:"job_name"`
	Cron     string `yaml:"cron"`
	Timezone string `yaml:"timezone,omitempty"`
	Enabled  bool   `yaml:"enabled"`
}

// LoadConfig loads configuration from file and environment variables
func LoadConfig(configPath string) (*Config, error) {
	// Start with minimal defaults (let YAML override)
	config := &Config{
		LLM: LLMConfig{
			BaseURL: "http://localhost:11434",
			Model:   "qwen3:14b",
			NumCtx:  8000,
			// Timeout will be set from YAML or default in GetTimeout()
		},
		OSRS: OSRSConfig{
			UserAgent:          "",
			MaxConcurrentCalls: 3,
			RateLimitDelayMs:   500,
			VolumeDataMaxItems: 2500,
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

	// Validate required fields
	if err := validateConfig(config); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	return config, nil
}

// loadYAMLFile loads configuration from a YAML file
func loadYAMLFile(path string, config *Config) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, config)
}

// loadEnvironmentVariables overrides config with environment variables
func loadEnvironmentVariables(config *Config) {
	if token := os.Getenv("DISCORD_TOKEN"); token != "" {
		config.Discord.Token = token
	}
	if channelID := os.Getenv("DISCORD_CHANNEL_ID"); channelID != "" {
		config.Discord.ChannelID = channelID
	}
	if guildID := os.Getenv("DISCORD_GUILD_ID"); guildID != "" {
		config.Discord.GuildID = guildID
	}
	if baseURL := os.Getenv("LLM_BASE_URL"); baseURL != "" {
		config.LLM.BaseURL = baseURL
	}
	if model := os.Getenv("LLM_MODEL"); model != "" {
		config.LLM.Model = model
	}
	if timeout := os.Getenv("LLM_TIMEOUT"); timeout != "" {
		config.LLM.Timeout = timeout
	}
	if level := os.Getenv("LOG_LEVEL"); level != "" {
		config.Logging.Level = level
	}
	if format := os.Getenv("LOG_FORMAT"); format != "" {
		config.Logging.Format = format
	}
	if userAgent := os.Getenv("OSRS_API_USER_AGENT"); userAgent != "" {
		config.OSRS.UserAgent = userAgent
	}
}

// validateConfig ensures required configuration is present
func validateConfig(config *Config) error {
	if config.OSRS.UserAgent == "" {
		return fmt.Errorf("a user agent string must be configured")
	}
	if config.Discord.Token == "" {
		return fmt.Errorf("discord token is required (set DISCORD_TOKEN environment variable)")
	}
	if config.Discord.ChannelID == "" {
		return fmt.Errorf("discord channel ID is required (set DISCORD_CHANNEL_ID environment variable)")
	}
	if len(config.Jobs) == 0 {
		return fmt.Errorf("at least one job must be configured")
	}

	return nil
}

// GetTimeout parses the LLM timeout string and returns a duration
func (c *LLMConfig) GetTimeout() time.Duration {
	if c.Timeout == "" {
		return 5 * time.Minute // default when not specified
	}
	duration, err := time.ParseDuration(c.Timeout)
	if err != nil {
		return 5 * time.Minute // default on parse error
	}
	return duration
}

// GetMaxConcurrentCalls returns the max concurrent API calls with bounds checking
func (c *OSRSConfig) GetMaxConcurrentCalls() int {
	if c.MaxConcurrentCalls < 1 {
		return 1
	}
	if c.MaxConcurrentCalls > 10 {
		return 10
	}
	return c.MaxConcurrentCalls
}

// GetRateLimitDelay returns the rate limit delay as a duration
func (c *OSRSConfig) GetRateLimitDelay() time.Duration {
	if c.RateLimitDelayMs < 100 {
		return 100 * time.Millisecond
	}
	return time.Duration(c.RateLimitDelayMs) * time.Millisecond
}

// GetJobModelConfig returns the effective model configuration for a job,
// merging job-specific overrides with the top-level LLM configuration
func (j *JobConfig) GetJobModelConfig(globalLLM *LLMConfig) llm.ModelConfig {
	// Start with the global model or a sensible default
	var modelConfig llm.ModelConfig
	if globalLLM != nil && globalLLM.Model != "" {
		switch globalLLM.Model {
		case "qwen3:14b":
			modelConfig = llm.CreateQwen3ModelConfig()
		default:
			modelConfig = llm.CreateDefaultModelConfig(globalLLM.Model)
		}
	} else {
		modelConfig = llm.CreateDefaultModelConfig("qwen3:14b")
	}

	// Apply job-specific overrides if they exist
	if j.Model != nil {
		if j.Model.Name != nil {
			modelConfig.Name = *j.Model.Name
		}
		if j.Model.NumCtx != nil {
			modelConfig.Options.NumCtx = *j.Model.NumCtx
		}
		if j.Model.Temperature != nil {
			modelConfig.Options.Temperature = *j.Model.Temperature
		}
		if j.Model.TopK != nil {
			modelConfig.Options.TopK = *j.Model.TopK
		}
		if j.Model.TopP != nil {
			modelConfig.Options.TopP = *j.Model.TopP
		}
		if j.Model.Seed != nil {
			modelConfig.Options.Seed = *j.Model.Seed
		}
		if j.Model.NumPredict != nil {
			modelConfig.Options.NumPredict = *j.Model.NumPredict
		}
		if j.Model.NumGPU != nil {
			modelConfig.Options.NumGPU = j.Model.NumGPU
		}
	}

	return modelConfig
}

// GetJobTimeout returns the effective timeout for a job,
// checking job-specific override first, then global LLM timeout
func (j *JobConfig) GetJobTimeout(globalLLM *LLMConfig) time.Duration {
	// Check job-specific timeout override first
	if j.Model != nil && j.Model.Timeout != nil {
		if duration, err := time.ParseDuration(*j.Model.Timeout); err == nil {
			return duration
		}
	}

	// Fall back to global LLM timeout
	if globalLLM != nil {
		return globalLLM.GetTimeout()
	}

	// Default timeout
	return 5 * time.Minute
}

// GetJobByName returns a job configuration by name, or nil if not found
func (c *Config) GetJobByName(name string) *JobConfig {
	for i := range c.Jobs {
		if c.Jobs[i].Name == name {
			return &c.Jobs[i]
		}
	}
	return nil
}
