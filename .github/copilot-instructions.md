r# Copilot Instructions for OSRS Flipping Repository

## Project Overview

This is a sophisticated Old School RuneScape (OSRS) trading analysis system built in Go, with Python prototyping components. The system analyzes Grand Exchange trading opportunities, provides LLM-powered insights, and delivers notifications via Discord bot.

## Architecture

- **Main Application**: `cmd/main.go` - CLI tool for one-time analysis and markdown output
- **Discord Bot**: `cmd/bot/main.go` - Scheduled Discord notifications
- **Core Packages**: Modular Go packages in `pkg/` for shared functionality
- **Python Prototypes**: Experimental/research code in `python/` directory
- **Configuration**: `config.yml` + `.env` environment variables
- **Build System**: `justfile` for common commands

## Critical Domain Knowledge

### OSRS Trading Model (ESSENTIAL UNDERSTANDING)

**Price Terminology** - This is fundamental to all trading logic:
- **`insta_sell_price`** = Price where **sell orders get filled instantly**
- **`insta_buy_price`** = Price where **buy orders get filled instantly**

**Trading Strategy**:
- **Buy orders**: Target the low `insta_sell_price` (instant sell price)
- **Sell orders**: Target the high `insta_buy_price` (instant buy price)
- **Profit**: The difference between these two prices (`margin_gp`)

**Key Insight**: This is NOT traditional bid/ask - it's the price at which orders execute immediately vs waiting in queue.

### Trading Metrics & Signals

Available metrics include:
- Volume metrics: `insta_buy_volume_*` / `insta_sell_volume_*` (20m/1h/24h timeframes)
- Price metrics: `avg_insta_buy_price_*` / `avg_insta_sell_price_*`
- Trend indicators: `*_trend_1h/24h/1w/1m` ('increasing'/'decreasing'/'flat')
- Margin metrics: `margin_gp`, `avg_margin_gp_*`

Key trading signals:
- **High Volume Opportunity**: Active two-way trading indicates liquid market
- **Volume Imbalance**: One-sided pressure suggests price movement
- **Margin Expansion**: Growing profit margins indicate good entry opportunity
- **Golden Opportunity**: High margin + high volume + recent data

## Code Style & Patterns

### Go Code Standards
- Use structured logging (`pkg/logging`)
- Implement graceful shutdown patterns for long-running services
- Follow Go naming conventions (PascalCase for exports, camelCase for internals)
- Use context.Context for cancellation and timeouts
- Error handling: wrap errors with context using `fmt.Errorf("operation failed: %w", err)`
- Configuration via structs with yaml tags

### Package Organization
- `pkg/config/` - Configuration loading and validation
- `pkg/osrs/` - OSRS API client and data structures
- `pkg/llm/` - LLM client and analysis functions
- `pkg/jobs/` - Core job execution and formatting
- `pkg/discord/` - Discord bot functionality
- `pkg/logging/` - Structured logging setup
- `pkg/scheduler/` - Cron job scheduling

### Configuration Management
- Primary config in `config.yml` with YAML struct tags
- Sensitive values via environment variables (Discord tokens, API keys)
- Use `pkg/config` package for loading and validation
- Support both CLI and container deployment modes

### Data Structures
- OSRS API responses use precise field names matching API (e.g., `insta_buy_price`, `insta_sell_price`)
- Time-based data uses Go time.Time with proper JSON marshaling
- Use typed enums for trend indicators ('increasing', 'decreasing', 'flat')
- Volume calculations require careful int64 handling for large numbers

## Development Workflow

### Build & Run Commands (via justfile)
- `just build` - Build both main and bot binaries + container
- `just run` - Run main CLI application
- `just bot` - Run Discord bot locally
- `just up` - Start containerized services
- `just down` - Stop containers
- `just logs` - View container logs

### Testing & Development
- Go tests use `*_test.go` pattern
- Python prototypes in `python/` for data exploration
- Use `go mod tidy` to manage dependencies
- Container deployments use multi-stage builds

### File Naming Conventions
- Go files: `snake_case.go`
- Test files: `*_test.go`
- Config files: `config.yml`, `.env`
- Output files: `output/Quick Flips_YYYY-MM-DD_HH-MM-SS.md`

## Key Dependencies & APIs

### External APIs
- OSRS API: Real-time trading data (rate limited, requires User-Agent)
- Ollama/LLM API: Local LLM for analysis (configurable endpoint)
- Discord API: Bot notifications

### Go Dependencies
- Configuration: `gopkg.in/yaml.v3`
- HTTP clients: `net/http` with custom rate limiting
- Logging: Structured JSON logging for Fluent Bit integration
- Discord: Discord Go library for bot functionality

### Python Dependencies (Prototyping)
- `pandas` for data analysis
- `pickle` for data persistence
- Custom modules in `python/llm/` and `python/utils/`

## Common Pitfalls & Important Notes

### OSRS API Considerations
- Respect rate limits (500ms delay between calls, max 3 concurrent)
- Always include proper User-Agent header
- Handle stale data (check `max_hours_since_update`)
- Volume calculations can involve large numbers (use int64)

### Trading Logic
- Never confuse `insta_buy_price` with traditional "bid" price
- Margin calculations: `insta_buy_price - insta_sell_price` (profit per unit)
- Volume trends are more important than absolute volumes
- Consider multiple timeframes (20m, 1h, 24h) for comprehensive analysis

### Configuration & Deployment
- Environment variables override config file values
- Discord tokens must be set via ENV vars, never committed
- Container deployments require proper health checks
- Graceful shutdown is critical for scheduled operations

### LLM Integration
- Prompts are designed for trading analysis context
- Timeouts are essential (20m default) for LLM calls
- JSON and text output formats supported
- Local Ollama deployment preferred for consistency

## Output Formats

The system generates:
- **Markdown files**: Timestamped trading analysis reports
- **Discord messages**: Formatted trading opportunities
- **JSON logs**: Structured logging for monitoring
- **Console output**: Human-readable CLI results

When working with output formatting, maintain consistency across formats and ensure Discord messages respect character limits and markdown formatting.
