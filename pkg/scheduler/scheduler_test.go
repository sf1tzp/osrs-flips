package scheduler

import (
	"context"
	"testing"
	"time"

	"osrs-flipping/pkg/config"
	"osrs-flipping/pkg/logging"

	"github.com/robfig/cron/v3"
)

// MockJobExecutor implements the JobExecutor interface for testing
type MockJobExecutor struct {
	executedJobs []string
	executions   int
}

func (m *MockJobExecutor) ExecuteJob(ctx context.Context, job config.JobConfig) error {
	m.executedJobs = append(m.executedJobs, job.Name)
	m.executions++
	return nil
}

func (m *MockJobExecutor) ExecuteAllJobs(ctx context.Context) error {
	m.executions++
	return nil
}

func TestSchedulerCronExpressions(t *testing.T) {
	logger := logging.NewLogger("info", "text")
	mockExecutor := &MockJobExecutor{}
	scheduler := NewScheduler(logger, mockExecutor)

	tests := []struct {
		name        string
		cronExpr    string
		expectError bool
		description string
	}{
		{
			name:        "valid_hourly",
			cronExpr:    "0 0 */1 * * *",
			expectError: false,
			description: "Every hour at minute 0",
		},
		{
			name:        "valid_every_2_hours",
			cronExpr:    "0 0 */2 * * *",
			expectError: false,
			description: "Every 2 hours",
		},
		{
			name:        "valid_specific_minutes",
			cronExpr:    "0 30 * * * *",
			expectError: false,
			description: "Every hour at minute 30",
		},
		{
			name:        "invalid_90_minutes",
			cronExpr:    "0 */90 * * * *",
			expectError: false, // This doesn't error, but doesn't work as expected
			description: "Invalid: Every 90 minutes (doesn't work as intended)",
		},
		{
			name:        "invalid_cron_format",
			cronExpr:    "invalid cron",
			expectError: true,
			description: "Invalid cron format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a test configuration
			cfg := &config.Config{
				Jobs: []config.JobConfig{
					{
						Name:    "Test Job",
						Enabled: true,
					},
				},
				Schedules: []config.ScheduleConfig{
					{
						JobName: "Test Job",
						Cron:    tt.cronExpr,
						Enabled: true,
					},
				},
			}

			err := scheduler.LoadJobs(cfg)
			if tt.expectError && err == nil {
				t.Errorf("Expected error for cron expression '%s', but got none", tt.cronExpr)
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error for cron expression '%s': %v", tt.cronExpr, err)
			}
		})
	}
}

func TestSchedulerNextRunTime(t *testing.T) {
	// Test that our corrected cron expression schedules correctly
	cronExpr := "0 0 */1 * * *" // Every hour at minute 0

	// Parse the cron expression directly
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		t.Fatalf("Failed to parse cron expression: %v", err)
	}

	// Test next run times
	now := time.Date(2025, 8, 7, 10, 30, 0, 0, time.UTC) // 10:30 AM
	next1 := schedule.Next(now)
	next2 := schedule.Next(next1)
	next3 := schedule.Next(next2)

	// Should run at 11:00, 12:00, 13:00
	expected1 := time.Date(2025, 8, 7, 11, 0, 0, 0, time.UTC)
	expected2 := time.Date(2025, 8, 7, 12, 0, 0, 0, time.UTC)
	expected3 := time.Date(2025, 8, 7, 13, 0, 0, 0, time.UTC)

	if !next1.Equal(expected1) {
		t.Errorf("First run: expected %v, got %v", expected1, next1)
	}
	if !next2.Equal(expected2) {
		t.Errorf("Second run: expected %v, got %v", expected2, next2)
	}
	if !next3.Equal(expected3) {
		t.Errorf("Third run: expected %v, got %v", expected3, next3)
	}

	// Verify it's exactly 1 hour between runs
	interval1 := next2.Sub(next1)
	interval2 := next3.Sub(next2)

	expectedInterval := time.Hour
	if interval1 != expectedInterval {
		t.Errorf("First interval: expected %v, got %v", expectedInterval, interval1)
	}
	if interval2 != expectedInterval {
		t.Errorf("Second interval: expected %v, got %v", expectedInterval, interval2)
	}
}

func TestInvalidNinetyMinuteCron(t *testing.T) {
	// Test the problematic "90 minute" cron expression
	cronExpr := "0 */90 * * * *"

	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		t.Fatalf("Failed to parse cron expression: %v", err)
	}

	// Test what this actually schedules
	now := time.Date(2025, 8, 7, 10, 0, 0, 0, time.UTC) // 10:00 AM
	next1 := schedule.Next(now)
	next2 := schedule.Next(next1)
	next3 := schedule.Next(next2)

	t.Logf("90-minute cron next runs: %v, %v, %v", next1, next2, next3)

	// The */90 in minutes field should only trigger at minute 0 (since 90 > 59)
	// So it should behave like "0 0 * * * *" (every hour at minute 0)
	expectedNext1 := time.Date(2025, 8, 7, 11, 0, 0, 0, time.UTC)
	if !next1.Equal(expectedNext1) {
		t.Logf("WARNING: */90 cron doesn't behave as expected. Next run: %v (expected %v)", next1, expectedNext1)
	}
}

func TestSchedulerJobExecution(t *testing.T) {
	logger := logging.NewLogger("info", "text")
	mockExecutor := &MockJobExecutor{}
	scheduler := NewScheduler(logger, mockExecutor)

	// Create test configuration
	cfg := &config.Config{
		Jobs: []config.JobConfig{
			{
				Name:    "Test Job 1",
				Enabled: true,
				Model: &config.JobModelConfig{
					Name:   stringPtr("test-model"),
					NumCtx: intPtr(8000),
				},
			},
			{
				Name:    "Test Job 2",
				Enabled: false, // disabled
				Model: &config.JobModelConfig{
					Name:   stringPtr("test-model"),
					NumCtx: intPtr(8000),
				},
			},
		},
		Schedules: []config.ScheduleConfig{
			{
				JobName: "Test Job 1",
				Cron:    "0 0 */1 * * *",
				Enabled: true,
			},
			{
				JobName: "Test Job 2",
				Cron:    "0 0 */1 * * *",
				Enabled: true, // schedule enabled but job disabled
			},
		},
	}

	err := scheduler.LoadJobs(cfg)
	if err != nil {
		t.Fatalf("Failed to load jobs: %v", err)
	}

	// Test manual job execution
	err = scheduler.ExecuteJob("Test Job 1")
	if err != nil {
		t.Errorf("Failed to execute job: %v", err)
	}

	// Give a moment for the goroutine to execute
	time.Sleep(100 * time.Millisecond)

	if mockExecutor.executions != 1 {
		t.Errorf("Expected 1 execution, got %d", mockExecutor.executions)
	}

	if len(mockExecutor.executedJobs) != 1 || mockExecutor.executedJobs[0] != "Test Job 1" {
		t.Errorf("Expected job 'Test Job 1' to be executed, got %v", mockExecutor.executedJobs)
	}

	// Test executing non-existent job
	err = scheduler.ExecuteJob("Non-existent Job")
	if err == nil {
		t.Error("Expected error when executing non-existent job")
	}
}

func TestSchedulerDisabledJobsAndSchedules(t *testing.T) {
	logger := logging.NewLogger("info", "text")
	mockExecutor := &MockJobExecutor{}
	scheduler := NewScheduler(logger, mockExecutor)

	// Create configuration with disabled jobs and schedules
	cfg := &config.Config{
		Jobs: []config.JobConfig{
			{
				Name:    "Enabled Job",
				Enabled: true,
			},
			{
				Name:    "Disabled Job",
				Enabled: false,
			},
		},
		Schedules: []config.ScheduleConfig{
			{
				JobName: "Enabled Job",
				Cron:    "0 0 */1 * * *",
				Enabled: true,
			},
			{
				JobName: "Enabled Job",
				Cron:    "0 30 */1 * * *",
				Enabled: false, // disabled schedule
			},
			{
				JobName: "Disabled Job",
				Cron:    "0 0 */1 * * *",
				Enabled: true, // enabled schedule but disabled job
			},
		},
	}

	err := scheduler.LoadJobs(cfg)
	if err != nil {
		t.Fatalf("Failed to load jobs: %v", err)
	}

	// Only one schedule should be active (Enabled Job with enabled schedule)
	entries := scheduler.cron.Entries()
	if len(entries) != 1 {
		t.Errorf("Expected 1 active cron entry, got %d", len(entries))
	}
}

// Helper functions for test configuration
func stringPtr(s string) *string { return &s }
func intPtr(i int) *int          { return &i }
