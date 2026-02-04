package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"osrs-flipping/pkg/collector"
)

// BucketConfig holds the configuration for a bucket size.
type BucketConfig struct {
	Size      string
	Retention time.Duration
}

var bucketConfigs = []BucketConfig{
	{Size: "5m", Retention: 7 * 24 * time.Hour},   // 7 days
	{Size: "1h", Retention: 30 * 24 * time.Hour},  // 30 days
	{Size: "24h", Retention: 365 * 24 * time.Hour}, // 365 days
}

// SyncStatus handles the --status command output.
type SyncStatus struct {
	repo *collector.Repository
}

// NewSyncStatus creates a new SyncStatus instance.
func NewSyncStatus(repo *collector.Repository) *SyncStatus {
	return &SyncStatus{repo: repo}
}

// PrintStatus prints the sync status for the specified bucket size(s).
func (s *SyncStatus) PrintStatus(ctx context.Context, bucketFilter string, showZeroItems bool) error {
	var configs []BucketConfig

	if bucketFilter != "" {
		// Find the specific bucket config
		for _, cfg := range bucketConfigs {
			if cfg.Size == bucketFilter {
				configs = []BucketConfig{cfg}
				break
			}
		}
		if len(configs) == 0 {
			return fmt.Errorf("unknown bucket size: %s (valid: 5m, 1h, 24h)", bucketFilter)
		}
	} else {
		configs = bucketConfigs
	}

	for i, cfg := range configs {
		if i > 0 {
			fmt.Println()
		}
		if err := s.printBucketStatus(ctx, cfg, showZeroItems); err != nil {
			return fmt.Errorf("print status for %s: %w", cfg.Size, err)
		}
	}

	return nil
}

func (s *SyncStatus) printBucketStatus(ctx context.Context, cfg BucketConfig, showZeroItems bool) error {
	// Get coverage stats
	stats, err := s.repo.GetSyncCoverageStats(ctx, cfg.Size)
	if err != nil {
		return fmt.Errorf("get coverage stats: %w", err)
	}

	// Get completeness distribution
	dist, err := s.repo.GetCompletenessDistribution(ctx, cfg.Size, cfg.Retention)
	if err != nil {
		return fmt.Errorf("get completeness distribution: %w", err)
	}

	// Print header
	retentionStr := formatRetention(cfg.Retention)
	fmt.Printf("SYNC STATUS: price_buckets_%s (%s retention)\n", cfg.Size, retentionStr)
	fmt.Println(strings.Repeat("=", 80))

	// Print coverage section
	fmt.Println("COVERAGE")
	fmt.Printf("  Total items:        %s\n", formatNumber(stats.TotalItems))

	coveragePct := float64(0)
	if stats.TotalItems > 0 {
		coveragePct = float64(stats.ItemsWithData) / float64(stats.TotalItems) * 100
	}
	fmt.Printf("  Items with data:    %s (%.1f%%)\n", formatNumber(stats.ItemsWithData), coveragePct)

	noCoveragePct := float64(0)
	if stats.TotalItems > 0 {
		noCoveragePct = float64(stats.ItemsWithNoData) / float64(stats.TotalItems) * 100
	}
	fmt.Printf("  Items with NO data: %s (%.1f%%)\n", formatNumber(stats.ItemsWithNoData), noCoveragePct)

	// Print data range section
	fmt.Println()
	fmt.Println("DATA RANGE")
	if stats.OldestBucket != nil {
		fmt.Printf("  Oldest: %s\n", stats.OldestBucket.Format("2006-01-02 15:04:05 UTC"))
	} else {
		fmt.Println("  Oldest: (no data)")
	}
	if stats.NewestBucket != nil {
		fmt.Printf("  Newest: %s\n", stats.NewestBucket.Format("2006-01-02 15:04:05 UTC"))
	} else {
		fmt.Println("  Newest: (no data)")
	}

	// Print completeness distribution
	fmt.Println()
	fmt.Printf("COMPLETENESS (%% of expected buckets)\n")

	// Find max count for bar scaling
	maxCount := max(
		dist.Complete90Plus,
		dist.Complete50to89,
		dist.Complete10to49,
		dist.CompleteLt10,
		dist.CompleteZero,
	)

	printCompletBar("90%+ complete", dist.Complete90Plus, maxCount)
	printCompletBar("50-89%", dist.Complete50to89, maxCount)
	printCompletBar("10-49%", dist.Complete10to49, maxCount)
	printCompletBar("<10%", dist.CompleteLt10, maxCount)
	printCompletBar("0% (no data)", dist.CompleteZero, maxCount)

	// Show zero items if requested
	if showZeroItems {
		fmt.Println()
		fmt.Println("ITEMS WITH ZERO DATA (first 50)")
		fmt.Println(strings.Repeat("-", 50))

		items, err := s.repo.GetItemsWithZeroData(ctx, cfg.Size, 50)
		if err != nil {
			return fmt.Errorf("get items with zero data: %w", err)
		}

		if len(items) == 0 {
			fmt.Println("  (none)")
		} else {
			for _, item := range items {
				fmt.Printf("  %6d  %s\n", item.ItemID, item.Name)
			}
		}
	}

	return nil
}

func printCompletBar(label string, count int64, maxCount int64) {
	const maxBars = 20

	numBars := 0
	if maxCount > 0 {
		numBars = int(float64(count) / float64(maxCount) * maxBars)
	}
	if count > 0 && numBars == 0 {
		numBars = 1 // Show at least one bar if there's any count
	}

	bar := strings.Repeat("\u2588", numBars) // Unicode full block character
	fmt.Printf("  %-15s %6s items %s\n", label+":", formatNumber(count), bar)
}

func formatNumber(n int64) string {
	// Add thousands separators
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}

	var result []byte
	for i, c := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result = append(result, ',')
		}
		result = append(result, byte(c))
	}
	return string(result)
}

func formatRetention(d time.Duration) string {
	days := int(d.Hours() / 24)
	if days >= 365 {
		years := days / 365
		if years == 1 {
			return "1y"
		}
		return fmt.Sprintf("%dy", years)
	}
	if days >= 30 {
		months := days / 30
		if months == 1 {
			return "30d"
		}
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dd", days)
}
