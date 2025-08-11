package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"osrs-flipping/pkg/config"
	"osrs-flipping/pkg/jobs"
	"osrs-flipping/pkg/logging"

	"github.com/robfig/cron/v3"
)

// Scheduler manages job scheduling and execution
type Scheduler struct {
	cron     *cron.Cron
	executor jobs.JobExecutor
	logger   *logging.Logger
	jobs     map[string]config.JobConfig
	mu       sync.RWMutex
}

// NewScheduler creates a new job scheduler
func NewScheduler(logger *logging.Logger, executor jobs.JobExecutor) *Scheduler {
	// Create cron with second precision and logging
	cronLogger := cron.VerbosePrintfLogger(logger.WithComponent("scheduler").Logger)
	c := cron.New(
		cron.WithSeconds(),
		cron.WithLogger(cronLogger),
		cron.WithChain(cron.Recover(cronLogger)),
	)

	return &Scheduler{
		cron:     c,
		executor: executor,
		logger:   logger,
		jobs:     make(map[string]config.JobConfig),
	}
}

// LoadJobs loads job configurations and sets up schedules
func (s *Scheduler) LoadJobs(cfg *config.Config) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Store job configurations
	for _, job := range cfg.Jobs {
		s.jobs[job.Name] = job
	}

	// Set up scheduled jobs
	for _, schedule := range cfg.Schedules {
		if !schedule.Enabled {
			s.logger.WithComponent("scheduler").WithField("job_name", schedule.JobName).Debug("Schedule disabled, skipping")
			continue
		}

		job, exists := s.jobs[schedule.JobName]
		if !exists {
			s.logger.WithComponent("scheduler").WithField("job_name", schedule.JobName).Error("Job not found for schedule")
			continue
		}

		if !job.Enabled {
			s.logger.WithComponent("scheduler").WithField("job_name", schedule.JobName).Debug("Job disabled, skipping schedule")
			continue
		}

		// Add cron job
		_, err := s.cron.AddFunc(schedule.Cron, func() {
			s.executeJob(job)
		})
		if err != nil {
			return fmt.Errorf("failed to add cron job for %s: %w", schedule.JobName, err)
		}

		s.logger.WithComponent("scheduler").WithFields(map[string]interface{}{
			"job_name": schedule.JobName,
			"cron":     schedule.Cron,
		}).Info("Scheduled job added")
	}

	return nil
}

// Start starts the scheduler
func (s *Scheduler) Start() {
	s.logger.WithComponent("scheduler").Info("Starting job scheduler")
	s.cron.Start()
}

// Stop stops the scheduler
func (s *Scheduler) Stop() {
	s.logger.WithComponent("scheduler").Info("Stopping job scheduler")
	ctx := s.cron.Stop()
	<-ctx.Done()
}

// ExecuteJob executes a job immediately (manual trigger)
func (s *Scheduler) ExecuteJob(jobName string) error {
	s.mu.RLock()
	job, exists := s.jobs[jobName]
	s.mu.RUnlock()

	if !exists {
		return fmt.Errorf("job %s not found", jobName)
	}

	go s.executeJob(job)
	return nil
}

// ExecuteAllJobs executes all enabled jobs immediately
func (s *Scheduler) ExecuteAllJobs() {
	s.mu.RLock()
	jobs := make([]config.JobConfig, 0, len(s.jobs))
	for _, job := range s.jobs {
		if job.Enabled {
			jobs = append(jobs, job)
		}
	}
	s.mu.RUnlock()

	s.logger.WithComponent("scheduler").WithField("job_count", len(jobs)).Info("Executing all enabled jobs")

	// Execute jobs sequentially to avoid API rate limiting
	for _, job := range jobs {
		s.executeJob(job)
		// Add delay between jobs to prevent API overload
		time.Sleep(5 * time.Second)
	}
}

// executeJob executes a single job with error handling
func (s *Scheduler) executeJob(job config.JobConfig) {
	timeout := 25 * time.Minute
	if job.Model != nil && job.Model.Timeout != nil {
		if jobTimeout, err := time.ParseDuration(*job.Model.Timeout); err != nil {
			timeout = jobTimeout
			s.logger.WithField("timeout", timeout).Info("parsed_timeout_from_job_config")
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	s.logger.WithComponent("scheduler").WithField("job_name", job.Name).
		WithField("model", job.Model.Name).
		WithField("ctx", job.Model.NumCtx).
		Info("Executing scheduled job")

	if err := s.executor.ExecuteJob(ctx, job); err != nil {
		s.logger.WithComponent("scheduler").WithField("job_name", job.Name).WithError(err).Error("Job execution failed")
	}
}

// GetJobNames returns a list of all configured job names
func (s *Scheduler) GetJobNames() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.jobs))
	for name := range s.jobs {
		names = append(names, name)
	}
	return names
}

// GetJobStatus returns the status of all jobs
func (s *Scheduler) GetJobStatus() map[string]bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := make(map[string]bool, len(s.jobs))
	for name, job := range s.jobs {
		status[name] = job.Enabled
	}
	return status
}

// IsRunning returns whether the scheduler is currently running
func (s *Scheduler) IsRunning() bool {
	return len(s.cron.Entries()) > 0
}
