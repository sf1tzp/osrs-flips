package jobs

import (
	"context"
	"fmt"
	"os"
	"time"

	"osrs-flipping/pkg/config"
	"osrs-flipping/pkg/discord"
	"osrs-flipping/pkg/llm"
	"osrs-flipping/pkg/logging"
	"osrs-flipping/pkg/osrs"
)

// Executor handles job execution and coordination
type Executor struct {
	config       *config.Config
	logger       *logging.Logger
	osrsAnalyzer *osrs.Analyzer
	llmClient    *llm.Client
	discordBot   *discord.Bot
	systemPrompt string
}

// NewExecutor creates a new job executor
func NewExecutor(cfg *config.Config, logger *logging.Logger, analyzer *osrs.Analyzer, llmClient *llm.Client, discordBot *discord.Bot) (*Executor, error) {
	executor := &Executor{
		config:       cfg,
		logger:       logger,
		osrsAnalyzer: analyzer,
		llmClient:    llmClient,
		discordBot:   discordBot,
	}

	// Load system prompt from files
	if err := executor.loadSystemPrompt(); err != nil {
		return nil, fmt.Errorf("failed to load system prompt: %w", err)
	}
	executor.logger.Debug("System Prompt")
	executor.logger.Debug(executor.systemPrompt)
	executor.logger.Debug("End System Prompt")

	return executor, nil
}

// loadSystemPrompt loads and combines prompt.md, signals.md, and example-output.md
func (e *Executor) loadSystemPrompt() error {
	var promptContent, signalsContent, exampleGoodContent, exampleBadContentTable, exampleBadContentNoLinks string

	// Load prompt.md
	if data, err := os.ReadFile("prompt.md"); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read prompt.md: %w", err)
		}
		e.logger.WithComponent("job_executor").Warn("prompt.md not found, using default prompt")
		promptContent = getDefaultPrompt()
	} else {
		promptContent = string(data)
	}

	// Load signals.md
	if data, err := os.ReadFile("signals.md"); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read signals.md: %w", err)
		}
		e.logger.WithComponent("job_executor").Warn("signals.md not found, using default signals")
		signalsContent = getDefaultSignals()
	} else {
		signalsContent = string(data)
	}

	// Load example-output.md
	if data, err := os.ReadFile("a-good-example-output.md"); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read a-good-example-output.md: %w", err)
		}
		e.logger.WithComponent("job_executor").Warn("a-good-example-output.md not found, using default example")
		exampleGoodContent = getDefaultExample()
	} else {
		exampleGoodContent = string(data)
	}

	// Load a-bad-example-output.md
	if data, err := os.ReadFile("a-bad-example-output-table.md"); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read a-bad-example-output-table.md: %w", err)
		}
		e.logger.WithComponent("job_executor").Warn("a-bad-example-output-table.md not found, using default example")
		exampleBadContentTable = getDefaultExample()
	} else {
		exampleBadContentTable = string(data)
	}

	// Load a-bad-example-output.md
	if data, err := os.ReadFile("a-bad-example-output-no-links.md"); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read a-bad-example-output-no-links.md: %w", err)
		}
		e.logger.WithComponent("job_executor").Warn("a-bad-example-output-no-links.md not found, using default example")
		exampleBadContentNoLinks = getDefaultExample()
	} else {
		exampleBadContentNoLinks = string(data)
	}

	// Combine prompts following the notebook pattern:
	// main prompt + signals section + example section
	e.systemPrompt = fmt.Sprintf(`%s

## Signals
<signals>
%s
</signals>

## Examples
<good-example reason="Wiki links and concise, expressive formatting. Includes the latest pricing as 'Strategy' and provides a timeframe estimate">
%s
</good-example>

<bad-example reason="no wiki links, no 'Strategy' section with latest pricing, and unecessary '---' separators between items">
%s
</bad-example>

<bad-example reason="the output should be a list of markdown stanzas, not a table">
%s
</bad-example>
`, promptContent, signalsContent, exampleGoodContent, exampleBadContentNoLinks, exampleBadContentTable)

	// e.logger.WithComponent("job_executor").WithField("prompt_length", len(e.systemPrompt)).Info("System prompt loaded successfully")
	// e.logger.WithComponent("job_executor").WithField("prompt", e.systemPrompt).Info("System prompt output")

	return nil
}

// ExecuteJobWithResult executes a job and returns a JobResult (used by JobRunner)
func (e *Executor) ExecuteJobWithResult(ctx context.Context, jobName string) (*JobResult, error) {
	startTime := time.Now()

	// Refresh OSRS data before each job execution to ensure fresh timestamps
	refreshCtx, refreshCancel := context.WithTimeout(ctx, 60*time.Second)
	defer refreshCancel()

	if err := e.osrsAnalyzer.LoadData(refreshCtx, true); err != nil {
		e.logger.WithFields(map[string]interface{}{
			"job_name": jobName,
			"error":    err.Error(),
		}).Warn("Failed to refresh OSRS data, using existing data")
		// Continue with existing data rather than failing the job
	}

	// Find the job configuration
	var jobConfig config.JobConfig
	found := false
	for _, job := range e.config.Jobs {
		if job.Name == jobName {
			jobConfig = job
			found = true
			break
		}
	}

	if !found {
		return &JobResult{
			JobName:   jobName,
			Success:   false,
			Error:     fmt.Errorf("job '%s' not found in configuration", jobName),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, nil
	}

	if !jobConfig.Enabled {
		return &JobResult{
			JobName:   jobName,
			Success:   false,
			Error:     fmt.Errorf("job '%s' is disabled", jobName),
			StartTime: startTime,
			EndTime:   time.Now(),
		}, nil
	}

	// Convert job filters to OSRS filter options
	filterOpts, err := e.convertFilters(jobConfig.Filters)
	if err != nil {
		endTime := time.Now()
		return &JobResult{
			JobName:   jobName,
			Success:   false,
			Error:     fmt.Errorf("failed to convert filters: %w", err),
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  endTime.Sub(startTime),
			JobConfig: jobConfig,
		}, nil
	}

	// Apply initial filters (price-based only) to get trading opportunities
	items, err := e.osrsAnalyzer.ApplyPrimaryFilter(filterOpts, true)
	if err != nil {
		endTime := time.Now()
		return &JobResult{
			JobName:   jobName,
			Success:   false,
			Error:     fmt.Errorf("failed to apply primary filters: %w", err),
			StartTime: startTime,
			EndTime:   endTime,
			Duration:  endTime.Sub(startTime),
			JobConfig: jobConfig,
		}, nil
	}

	// Always load volume data if items are available
	if len(items) > 0 {
		volumeCtx, cancel := context.WithTimeout(ctx, 20*time.Minute)
		defer cancel()

		// Extract item IDs for volume loading
		itemIDs := make([]int, len(items))
		for i, item := range items {
			itemIDs[i] = item.ItemID
		}

		maxVolumeItems := e.config.OSRS.VolumeDataMaxItems
		if len(items) < maxVolumeItems {
			maxVolumeItems = len(items)
		}

		if err := e.osrsAnalyzer.LoadVolumeData(volumeCtx, itemIDs, maxVolumeItems); err != nil {
			e.logger.WithFields(map[string]interface{}{
				"job_name":   jobName,
				"error":      err.Error(),
				"item_count": len(itemIDs),
			}).Warn("Volume data loading failed (continuing without)")
		} else {
			// Get the items with volume data from the analyzer
			itemsWithVolume := e.osrsAnalyzer.GetItemsWithVolume(itemIDs)

			// Apply secondary filters (volume-based)
			filteredItems, err := e.osrsAnalyzer.ApplySecondaryFilter(itemsWithVolume, filterOpts, true)
			if err != nil {
				e.logger.WithFields(map[string]interface{}{
					"job_name":   jobName,
					"error":      err.Error(),
					"item_count": len(itemsWithVolume),
				}).Warn("Secondary filtering failed, using items without volume filters")
			} else {
				items = filteredItems
			}
		}
	}

	// Limit items for output
	if jobConfig.Output.MaxItems > 0 && len(items) > jobConfig.Output.MaxItems {
		items = items[:jobConfig.Output.MaxItems]
	}

	// Handle case where no items remain after filtering
	if len(items) == 0 {
		endTime := time.Now()
		duration := endTime.Sub(startTime)

		e.logger.WithFields(map[string]interface{}{
			"job_name": jobName,
			"duration": duration,
		}).Warn("No items met filtering criteria")

		return &JobResult{
			JobName:    jobName,
			Success:    true, // Job ran successfully, just no results
			StartTime:  startTime,
			EndTime:    endTime,
			Duration:   duration,
			ItemsFound: 0,
			Analysis:   "No items met the filtering criteria this for job RIP",
			RawItems:   items,
			JobConfig:  jobConfig,
		}, nil
	}

	var analysis string
	var jobSuccess = true

	// Always generate LLM analysis if items are available
	if len(items) > 0 {
		e.logger.WithFields(map[string]interface{}{
			"job_name":   jobName,
			"item_count": len(items),
		}).Info("Generating LLM analysis")

		// Check LLM connection
		connCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		if err := e.llmClient.CheckConnection(connCtx); err != nil {
			e.logger.WithFields(map[string]interface{}{
				"job_name": jobName,
				"error":    err.Error(),
			}).Warn("LLM connection failed, skipping analysis")
			analysis = fmt.Sprintf("LLM analysis failed: %v", err)
			jobSuccess = false
		} else {
			// Use our notebook pattern analysis method
			analysisResult, err := e.generateAnalysis(ctx, items, jobConfig)
			if err != nil {
				e.logger.WithFields(map[string]interface{}{
					"job_name": jobName,
					"error":    err.Error(),
				}).Error("LLM analysis failed")
				analysis = fmt.Sprintf("LLM analysis failed: %v", err)
				jobSuccess = false
			} else {
				e.logger.WithFields(map[string]interface{}{
					"job_name":   jobName,
					"item_count": len(items),
				}).Info("LLM analysis completed successfully")
				analysis = analysisResult
			}
		}
	}

	endTime := time.Now()
	duration := endTime.Sub(startTime)

	result := &JobResult{
		JobName:    jobName,
		Success:    jobSuccess,
		StartTime:  startTime,
		EndTime:    endTime,
		Duration:   duration,
		ItemsFound: len(items),
		Analysis:   analysis,
		RawItems:   items,
		JobConfig:  jobConfig,
	}

	e.logger.WithFields(map[string]interface{}{
		"job_name":   jobName,
		"duration":   duration,
		"item_count": len(items),
		"success":    jobSuccess,
	}).Info("Job completed")
	return result, nil
}

// convertFilters converts job filter config to OSRS filter options
func (e *Executor) convertFilters(filters config.FilterConfig) (osrs.FilterOptions, error) {
	opts := osrs.FilterOptions{}

	if filters.MarginMin != nil {
		opts.MarginMin = filters.MarginMin
	}
	if filters.MarginPctMin != nil {
		opts.MarginPctMin = filters.MarginPctMin
	}
	if filters.BuyLimitMin != nil {
		opts.BuyLimitMin = filters.BuyLimitMin
	}
	if filters.BuyLimitMax != nil {
		opts.BuyLimitMax = filters.BuyLimitMax
	}
	if filters.InstaSellPriceMin != nil {
		opts.InstaSellPriceMin = filters.InstaSellPriceMin
	}
	if filters.InstaSellPriceMax != nil {
		opts.InstaSellPriceMax = filters.InstaSellPriceMax
	}
	if filters.Volume1hMin != nil {
		opts.Volume1hMin = filters.Volume1hMin
	}
	if filters.Volume24hMin != nil {
		opts.Volume24hMin = filters.Volume24hMin
	}
	if filters.MaxHoursSinceUpdate != nil {
		opts.MaxHoursSinceUpdate = filters.MaxHoursSinceUpdate
	}

	// Set default sorting if not specified
	if filters.SortBy != "" {
		opts.SortBy = filters.SortBy
	} else {
		opts.SortBy = "margin_gp"
	}

	if filters.SortDesc != nil {
		opts.SortDesc = *filters.SortDesc
	} else {
		opts.SortDesc = true
	}

	if filters.Limit != nil {
		opts.Limit = *filters.Limit
	} else {
		opts.Limit = 50 // reasonable default
	}

	return opts, nil
}

// generateAnalysis generates LLM analysis for the items
// Following the notebook pattern: configure model, generate response, clean response
func (e *Executor) generateAnalysis(ctx context.Context, items []osrs.ItemData, jobConfig config.JobConfig) (string, error) {
	// Get job-specific model configuration (merges job overrides with global config)
	modelConfig := jobConfig.GetJobModelConfig(&e.config.LLM)

	// Get job-specific timeout
	timeout := jobConfig.GetJobTimeout(&e.config.LLM)

	// Create a context with the job-specific timeout
	analysisCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Format items for LLM input - this is our "user_prompt" equivalent
	// Todo: Attach this file to a discord message
	userPrompt := llm.FormatItemsForAnalysisV2(items, len(items))

	// Temporarily log to verify volume data is now included
	e.logger.Debug("User prompt with volume data:")
	e.logger.Debug(userPrompt)

	// Get response from LLM (matching notebook's get_generate_response pattern)
	response, err := llm.GetGenerateResponse(analysisCtx, e.llmClient, modelConfig, e.systemPrompt, userPrompt)
	if err != nil {
		return "", fmt.Errorf("LLM generation failed: %w", err)
	}

	// Clean the response by removing thinking tags (matching notebook pattern)
	cleanedResponse := llm.RemoveThinkingTags(response.Response)

	e.logger.WithLLM().WithFields(map[string]interface{}{
		"job_name":        jobConfig.Name,
		"items_analyzed":  len(items),
		"response_length": len(cleanedResponse),
		"duration_sec":    response.TotalDuration.Seconds(),
		"timeout":         timeout,
		"model":           modelConfig.Name,
		"context_size":    modelConfig.Options.NumCtx,
		"temperature":     modelConfig.Options.Temperature,
	}).Info("LLM analysis completed")

	return cleanedResponse, nil
}

// getDefaultPrompt returns a basic trading prompt if prompt.md is not found
func getDefaultPrompt() string {
	return `You are an OSRS trading analysis expert. Analyze the provided trading data and provide actionable trading recommendations.

Focus on:
- High-margin opportunities with good volume
- Risk assessment based on price trends
- Clear buy/sell prices and timeframes
- Profit potential and investment requirements

Remember: insta_sell_price = what you can BUY at, insta_buy_price = what you can SELL at.`
}

// getDefaultSignals returns basic signals info if signals.md is not found
func getDefaultSignals() string {
	return `# OSRS Trading Signals

## Key Metrics:
- margin_gp: Profit per item (insta_buy_price - insta_sell_price)
- margin_pct: Profit percentage
- Volume metrics: insta_buy_volume_1h, insta_sell_volume_1h
- Price trends: insta_buy_price_trend_24h, insta_sell_price_trend_24h

## Risk Factors:
- Low volume = harder to execute trades
- Stale prices = data may be outdated
- High price items = more capital required`
}

// getDefaultExample returns a basic example if example-output.md is not found
func getDefaultExample() string {
	return `# OSRS Trading Analysis: Sample Output

## 🎯 Top Trading Recommendations

### 1. Dragon platebody ⭐⭐⭐
- **Current margin**: 45,000 GP (12.3%)
- **Investment**: 365,000 GP
- **Strategy**: Buy at 365k, sell at 410k
- **Timeframe**: 2-4 hours
- **Rationale**: High volume item with consistent demand

### 2. Bandos chestplate ⭐⭐
- **Current margin**: 180,000 GP (8.5%)
- **Investment**: 2,120,000 GP
- **Strategy**: Buy at 2.12M, sell at 2.30M
- **Timeframe**: 4-8 hours
- **Rationale**: Popular PvM gear with steady liquidity

## 🔍 Market Analysis
Current market shows strong demand for mid-tier combat equipment. Volume is healthy across most recommendations.`
}
