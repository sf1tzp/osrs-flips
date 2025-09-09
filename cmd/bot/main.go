package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"osrs-flipping/pkg/config"
	"osrs-flipping/pkg/discord"
	"osrs-flipping/pkg/jobs"
	"osrs-flipping/pkg/logging"
	"osrs-flipping/pkg/scheduler"
)

const VERSION = "0.0.11"

func main() {
	// Load configuration with Discord validation
	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	logger := logging.NewLogger(cfg.Logging.Level, cfg.Logging.Format)

	logger.WithComponent("main").WithField("version", VERSION).Info("starting_osrs_flips_bot")

	// Initialize job runner (unified with main program)
	jobRunner, err := jobs.NewJobRunner(cfg)
	if err != nil {
		logger.WithComponent("main").WithError(err).Fatal("Failed to create job runner")
	}

	// Load base OSRS data
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	logger.WithOSRS().Info("Loading OSRS trading data")
	if err := jobRunner.LoadData(ctx); err != nil {
		logger.WithOSRS().WithError(err).Fatal("Failed to load OSRS data")
	}
	logger.WithOSRS().Info("OSRS data loaded successfully")

	// Initialize output formatter
	formatter := jobs.NewOutputFormatter()

	// Initialize Discord bot
	var discordBot *discord.Bot
	if cfg.Discord.Token != "" && cfg.Discord.ChannelID != "" {
		discordBot, err = discord.NewBot(&cfg.Discord, logger)
		if err != nil {
			logger.WithDiscord().WithError(err).Fatal("Failed to create Discord bot")
		}

		// Start Discord bot
		botCtx, botCancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer botCancel()

		if err := discordBot.Start(botCtx); err != nil {
			logger.WithDiscord().WithError(err).Fatal("Failed to start Discord bot")
		}
		logger.WithDiscord().Info("Discord bot started successfully")

		// Send startup message
		if _, err := discordBot.SendMessage(fmt.Sprintf("🏰 **osrs-flips v%s** has logged in.", VERSION)); err != nil {
			logger.WithDiscord().WithError(err).Warn("Failed to send startup message")
		}
	} else {
		logger.WithComponent("main").Warn("Discord configuration missing - bot will run without Discord integration")
	}

	// Initialize bot executor that wraps the job runner
	botExecutor := &BotExecutor{
		jobRunner:  jobRunner,
		formatter:  formatter,
		discordBot: discordBot,
		logger:     logger,
	}

	// Initialize and start scheduler
	sched := scheduler.NewScheduler(logger, botExecutor)
	if err := sched.LoadJobs(cfg); err != nil {
		logger.WithComponent("scheduler").WithError(err).Fatal("Failed to load job schedules")
	}
	sched.Start()

	logger.WithComponent("main").WithFields(map[string]interface{}{
		"jobs_loaded":      len(cfg.Jobs),
		"schedules_active": len(cfg.Schedules),
		"discord_enabled":  discordBot != nil,
	}).Info("osrs-flips fully initialized")

	// // Execute all jobs once on startup
	// logger.WithComponent("main").Info("Running initial job execution")
	// sched.ExecuteAllJobs()

	// Set up graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	logger.WithComponent("main").Info("Shutdown signal received, gracefully stopping...")

	// Stop scheduler
	sched.Stop()

	// Stop Discord bot
	if discordBot != nil {
		if _, err := discordBot.SendMessage("☠️ Oh dear, **osrs-flips** has died. a q p "); err != nil {
			logger.WithDiscord().WithError(err).Warn("Failed to send shutdown message")
		}

		// Give a moment for the message to send
		time.Sleep(2 * time.Second)

		if err := discordBot.Stop(); err != nil {
			logger.WithDiscord().WithError(err).Error("Error stopping Discord bot")
		}
	}

	logger.WithComponent("main").Info("osrs-flips shutdown complete")
}

// BotExecutor wraps the JobRunner to provide the interface expected by the scheduler
type BotExecutor struct {
	jobRunner  *jobs.JobRunner
	formatter  *jobs.OutputFormatter
	discordBot *discord.Bot
	logger     *logging.Logger
}

// ExecuteJob runs a job and posts results to Discord
func (be *BotExecutor) ExecuteJob(ctx context.Context, job config.JobConfig) error {
	// Run the job using the unified job runner
	result, err := be.jobRunner.RunJob(ctx, job.Name)
	if err != nil {
		be.logger.WithComponent("bot").WithError(err).Error("Job execution failed")
		return err
	}

	// Log the result
	if result.Success {
		be.logger.WithComponent("bot").WithFields(map[string]interface{}{
			"job":      job.Name,
			"duration": result.Duration,
			"items":    result.ItemsFound,
		}).Info("Job completed successfully")
	} else {
		be.logger.WithComponent("bot").WithError(result.Error).Error("Job failed")

		if result.Error == nil {
			result.Error = fmt.Errorf("job %s failed with unknown error", job.Name)
		}

		// Send error message to Discord instead of success message
		if be.discordBot != nil {
			if err := be.discordBot.SendError(job.Name, result.Error); err != nil {
				be.logger.WithDiscord().WithError(err).Error("Failed to send error message to Discord")
			}
		}
	}

	// Send to Discord if available
	if be.discordBot != nil {
		// discordMessage := be.formatter.FormatForDiscord(result)
		discordMessage := result.Analysis

		// Handle empty message case (no items found)
		if discordMessage == "" && result.ItemsFound == 0 {
			discordMessage = fmt.Sprintf("📊 **%s**\n\nNo items met the filtering criteria. Consider adjusting your filters for more results.", result.JobName)
		}

		footerText := fmt.Sprintf("Generated with %s using data from https://oldschool.runescape.wiki/w/RuneScape:Real-time_Prices", *result.JobConfig.Model.Name)

		if err := be.discordBot.SendLongAnalysis(result.JobName, discordMessage, footerText, result.ItemsFound); err != nil {
			be.logger.WithDiscord().WithError(err).Error("Failed to send job results to Discord")
			return err
		}
	}

	return nil
}

// ExecuteAllJobs runs all enabled jobs
func (be *BotExecutor) ExecuteAllJobs(ctx context.Context) error {
	results, err := be.jobRunner.RunAllJobs(ctx)
	if err != nil {
		return err
	}

	// Process each result
	for _, result := range results {
		if result.Success {
			be.logger.WithComponent("bot").WithFields(map[string]interface{}{
				"job":      result.JobName,
				"duration": result.Duration,
				"items":    result.ItemsFound,
			}).Info("Job completed successfully")
		} else {
			be.logger.WithComponent("bot").WithError(result.Error).Error("Job failed")

			// Send error message to Discord for failed jobs
			if be.discordBot != nil {
				if err := be.discordBot.SendError(result.JobName, result.Error); err != nil {
					be.logger.WithDiscord().WithError(err).Error("Failed to send error message to Discord")
				}
			}
			continue // Skip sending success message for failed jobs
		}

		// Send to Discord if available
		if be.discordBot != nil {
			// discordMessage := be.formatter.FormatForDiscord(result)
			discordMessage := fmt.Sprintf("> %s \n\n %s", result.JobConfig.Description, result.Analysis)

			// Handle empty message case (no items found)
			if discordMessage == "" && result.ItemsFound == 0 {
				discordMessage = fmt.Sprintf("📊 **%s**\n\nNo items met the filtering criteria. Consider adjusting your filters for more results.", result.JobName)
			}

			footerText := fmt.Sprintf("Generated with %s using data from https://oldschool.runescape.wiki/w/RuneScape:Real-time_Prices", *result.JobConfig.Model.Name)
			if err := be.discordBot.SendLongAnalysis(result.JobName, discordMessage, footerText, result.ItemsFound); err != nil {
				be.logger.WithDiscord().WithError(err).Error("Failed to send job results to Discord")
				return err
			}
		}
	}

	return nil
}
