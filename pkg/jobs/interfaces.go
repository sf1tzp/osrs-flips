package jobs

import (
	"context"
	"osrs-flipping/pkg/config"
)

// JobExecutor defines the interface for job execution
type JobExecutor interface {
	ExecuteJob(ctx context.Context, job config.JobConfig) error
	ExecuteAllJobs(ctx context.Context) error
}
