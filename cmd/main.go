package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"osrs-flipping/pkg/config"
	"osrs-flipping/pkg/jobs"
)

func main() {
	fmt.Println("🚀 OSRS Trade Analysis - Go Edition")
	fmt.Println("=====================================")

	// Load configuration from config.yml and environment variables (.env file)
	cfg, err := config.LoadConfigForMain("config.yml")
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	fmt.Printf("📝 Configuration loaded:\n")
	fmt.Printf("   Ollama URL: %s\n", cfg.LLM.BaseURL)
	fmt.Printf("   Ollama Timeout: %s\n", cfg.LLM.Timeout)
	fmt.Printf("   Model: %s\n", cfg.LLM.Model)
	fmt.Printf("   Log Level: %s\n", cfg.Logging.Level)
	fmt.Printf("   Jobs configured: %d\n", len(cfg.Jobs))

	// Create job runner
	jobRunner, err := jobs.NewJobRunner(cfg)
	if err != nil {
		log.Fatalf("Failed to create job runner: %v", err)
	}

	// Load base OSRS data
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	fmt.Println("\n📊 Loading OSRS data...")
	if err := jobRunner.LoadData(ctx); err != nil {
		log.Fatalf("Failed to load OSRS data: %v", err)
	}

	// Create output formatter
	formatter := jobs.NewOutputFormatter()

	// Run all enabled jobs
	fmt.Println("\n🔍 Running all enabled jobs...")

	jobCtx, jobCancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer jobCancel()

	results, err := jobRunner.RunAllJobs(jobCtx)
	if err != nil {
		log.Fatalf("Failed to run jobs: %v", err)
	}

	// Process results
	for _, result := range results {
		// Print to terminal
		terminalOutput := formatter.FormatForTerminal(result)
		fmt.Print(terminalOutput)

		// Save to markdown file if successful
		if result.Success {
			filename := fmt.Sprintf("%s_%s.md",
				result.JobName,
				result.StartTime.Format("2006-01-02_15-04-05"))

			// Clean filename
			filename = filepath.Clean(filename)
			filename = fmt.Sprintf("output/%s", filename)

			// Create output directory if it doesn't exist
			if err := os.MkdirAll("output", 0755); err != nil {
				log.Printf("Failed to create output directory: %v", err)
				continue
			}

			markdownOutput := formatter.FormatForMarkdown(result)
			if err := os.WriteFile(filename, []byte(markdownOutput), 0644); err != nil {
				log.Printf("Failed to write markdown file %s: %v", filename, err)
			} else {
				fmt.Printf("📄 Results saved to: %s\n", filename)
			}
		}
	}

	// Summary
	successCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		}
	}

	fmt.Println("\n✅ Job execution complete!")
	fmt.Printf("   Total jobs: %d\n", len(results))
	fmt.Printf("   Successful: %d\n", successCount)
	fmt.Printf("   Failed: %d\n", len(results)-successCount)
}
