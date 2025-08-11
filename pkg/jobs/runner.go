package jobs

import (
	"context"
	"fmt"
	"time"

	"osrs-flipping/pkg/config"
	"osrs-flipping/pkg/llm"
	"osrs-flipping/pkg/logging"
	"osrs-flipping/pkg/osrs"
)

// JobResult represents the output of a job execution
type JobResult struct {
	JobName    string
	Success    bool
	Error      error
	StartTime  time.Time
	EndTime    time.Time
	Duration   time.Duration
	ItemsFound int
	Analysis   string
	RawItems   []osrs.ItemData
	JobConfig  config.JobConfig
}

// JobRunner handles the execution of trading analysis jobs
type JobRunner struct {
	config   *config.Config
	executor *Executor
	logger   *logging.Logger
}

// NewJobRunner creates a new job runner with the given configuration
func NewJobRunner(cfg *config.Config) (*JobRunner, error) {
	// Create analyzer
	analyzer := osrs.NewAnalyzer(cfg.OSRS.UserAgent)

	// Parse LLM timeout
	timeout, err := time.ParseDuration(cfg.LLM.Timeout)
	if err != nil {
		timeout = 5 * time.Minute
		// We'll log this after creating the logger
	}

	// Create LLM client
	llmClient := llm.NewClient(cfg.LLM.BaseURL, timeout)

	// Create logger for the executor
	logger := logging.NewLogger(cfg.Logging.Level, cfg.Logging.Format)

	// Log timeout warning if needed
	if err != nil {
		logger.WithFields(map[string]interface{}{
			"error":           err.Error(),
			"default_timeout": timeout.String(),
		}).Warn("Invalid LLM timeout format, using default")
	}

	// Create executor with the notebook pattern
	executor, err := NewExecutor(cfg, logger, analyzer, llmClient, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create executor: %w", err)
	}

	return &JobRunner{
		config:   cfg,
		executor: executor,
		logger:   logger,
	}, nil
}

// LoadData loads the base OSRS data (should be called once at startup)
func (jr *JobRunner) LoadData(ctx context.Context) error {
	jr.logger.Info("Loading OSRS base data")
	return jr.executor.osrsAnalyzer.LoadData(ctx, true)
}

// RefreshData refreshes the OSRS data (for scheduled updates)
func (jr *JobRunner) RefreshData(ctx context.Context) error {
	jr.logger.Info("Refreshing OSRS base data")
	return jr.executor.osrsAnalyzer.LoadData(ctx, true)
}

// RunJob executes a specific job by name and returns the result
func (jr *JobRunner) RunJob(ctx context.Context, jobName string) (*JobResult, error) {
	// Delegate to the executor which has the notebook pattern
	return jr.executor.ExecuteJobWithResult(ctx, jobName)
}

// RunAllJobs executes all enabled jobs and returns their results
func (jr *JobRunner) RunAllJobs(ctx context.Context) ([]*JobResult, error) {
	var results []*JobResult

	for _, job := range jr.config.Jobs {
		if !job.Enabled {
			jr.logger.WithFields(map[string]interface{}{
				"job_name": job.Name,
			}).Info("Skipping disabled job")
			continue
		}
		jr.logger.WithComponent("JobRunner").WithField("job_name", job.Name).
			WithField("model", job.Model.Name).
			WithField("ctx", job.Model.NumCtx).
			Info("Executing scheduled job")

		result, err := jr.RunJob(ctx, job.Name)
		if err != nil {
			jr.logger.WithFields(map[string]interface{}{
				"job_name": job.Name,
				"error":    err.Error(),
			}).Error("Job execution failed")
		}

		results = append(results, result)
	}

	return results, nil
}
