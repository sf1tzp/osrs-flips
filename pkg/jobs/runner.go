package jobs

import (
	"context"
	"fmt"
	"os"
	"time"

	"osrs-flipping/pkg/config"
	"osrs-flipping/pkg/database"
	"osrs-flipping/pkg/llm"
	"osrs-flipping/pkg/logging"
	"osrs-flipping/pkg/osrs"
	"osrs-flipping/pkg/storage"
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
	// Create logger for the executor
	logger := logging.NewLogger(cfg.Logging.Level, cfg.Logging.Format)

	// Create analyzer
	analyzer := osrs.NewAnalyzer(cfg.OSRS.UserAgent)

	// Check if database is configured and set up hybrid data source
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		logger.WithComponent("JobRunner").Info("DATABASE_URL found, setting up hybrid data source")

		// Connect to database
		dbConfig, err := database.ConfigFromEnv()
		if err != nil {
			logger.WithComponent("JobRunner").WithError(err).Warn("Failed to load database config, using API-only mode")
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			db, err := database.Connect(ctx, dbConfig)
			cancel()

			if err != nil {
				logger.WithComponent("JobRunner").WithError(err).Warn("Failed to connect to database, using API-only mode")
			} else {
				// Create the hybrid data source
				queryRepo := storage.NewQueryRepository(db.Pool)
				apiSource := osrs.NewAPIDataSourceWithClient(analyzer.GetClient())
				dbSource := osrs.NewDBDataSource(queryRepo, analyzer.GetClient(), 5*time.Minute)
				hybridSource := osrs.NewHybridDataSource(dbSource, apiSource)

				analyzer.SetDataSource(hybridSource)
				logger.WithComponent("JobRunner").Info("Hybrid data source configured (DB + API fallback)")
			}
		}
	} else {
		logger.WithComponent("JobRunner").Info("No DATABASE_URL found, using API-only mode")
	}

	// Parse LLM timeout
	timeout, err := time.ParseDuration(cfg.LLM.Timeout)
	if err != nil {
		timeout = 5 * time.Minute
		// We'll log this after creating the logger
	}

	// Create LLM client
	llmClient := llm.NewClient(cfg.LLM.BaseURL, timeout)

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
	return jr.executor.osrsAnalyzer.LoadDataFromSource(ctx, true)
}

// RefreshData refreshes the OSRS data (for scheduled updates)
func (jr *JobRunner) RefreshData(ctx context.Context) error {
	jr.logger.Info("Refreshing OSRS base data")
	return jr.executor.osrsAnalyzer.LoadDataFromSource(ctx, true)
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
