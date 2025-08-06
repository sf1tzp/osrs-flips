package jobs

import (
	"fmt"
	"strings"
	"time"
)

// OutputFormatter handles formatting job results for different outputs
type OutputFormatter struct{}

// NewOutputFormatter creates a new output formatter
func NewOutputFormatter() *OutputFormatter {
	return &OutputFormatter{}
}

// FormatForTerminal formats job results for terminal output
func (of *OutputFormatter) FormatForTerminal(result *JobResult) string {
	var output strings.Builder

	// Header
	output.WriteString(fmt.Sprintf("\n🎯 Job Results: %s\n", result.JobName))
	output.WriteString(strings.Repeat("=", 60) + "\n")

	if !result.Success {
		output.WriteString(fmt.Sprintf("❌ Job failed: %v\n", result.Error))
		return output.String()
	}

	// Job info
	output.WriteString(fmt.Sprintf("📊 Status: Success | Duration: %v | Items: %d\n",
		result.Duration.Truncate(time.Millisecond), result.ItemsFound))

	if result.JobConfig.Description != "" {
		output.WriteString(fmt.Sprintf("📝 Description: %s\n", result.JobConfig.Description))
	}

	output.WriteString("\n")

	// LLM Analysis
	if result.Analysis != "" {
		output.WriteString("🤖 LLM Analysis:\n")
		output.WriteString(strings.Repeat("-", 60) + "\n")
		output.WriteString(result.Analysis)
		output.WriteString("\n" + strings.Repeat("-", 60) + "\n")
	}

	return output.String()
}

// FormatForMarkdown formats job results for markdown file output
func (of *OutputFormatter) FormatForMarkdown(result *JobResult) string {
	var output strings.Builder

	// LLM Analysis
	if result.Analysis != "" {
		output.WriteString(result.Analysis)
		output.WriteString("\n\n")
	}

	return output.String()
}

// FormatForDiscord formats job results for Discord message
func (of *OutputFormatter) FormatForDiscord(result *JobResult) string {
	var output strings.Builder

	// LLM Analysis (using smart text handling for Discord)
	if result.Analysis != "" {
		analysis := result.Analysis
		const discordLimit = 2000

		// Truncate analysis if it exceeds Discord's character limit
		if len(analysis) > discordLimit {
			analysis = analysis[:discordLimit-3] + "..."
		}
		output.WriteString(analysis)
	}

	return output.String()
}
