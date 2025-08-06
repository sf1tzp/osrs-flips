# OSRS Trading Analysis - Unified Architecture

## Overview

The OSRS Trading Analysis system now uses a unified architecture where both the main program and Discord bot share the same core job execution engine.

## Architecture

```
┌─────────────────┐    ┌─────────────────┐
│   cmd/main.go   │    │ cmd/bot/main.go │
│                 │    │                 │
│ - Runs once     │    │ - Runs on cron  │
│ - Saves to MD   │    │ - Posts to      │
│ - Prints output │    │   Discord       │
└─────────────────┘    └─────────────────┘
         │                       │
         └───────┬───────────────┘
                 │
         ┌───────▼────────┐
         │ pkg/jobs/      │
         │                │
         │ JobRunner      │ ◄── Core job execution
         │ OutputFormatter│ ◄── Multi-format output
         └────────────────┘
                 │
         ┌───────▼────────┐
         │ pkg/osrs/      │
         │ pkg/llm/       │ ◄── OSRS API + LLM
         │ pkg/config/    │ ◄── Configuration
         └────────────────┘
```

## Core Components

### 1. **JobRunner** (`pkg/jobs/runner.go`)
- **Shared by both main and bot**
- Executes trading analysis jobs
- Handles OSRS API calls with rate limiting
- Manages LLM integration for analysis
- Returns structured `JobResult` objects

### 2. **OutputFormatter** (`pkg/jobs/formatter.go`)
- **Formats results for different outputs:**
  - `FormatForTerminal()` - Human-readable terminal output
  - `FormatForMarkdown()` - Detailed markdown files
  - `FormatForDiscord()` - Discord-optimized messages

### 3. **Configuration System** (`pkg/config/`)
- **Unified config.yml** for both programs
- **Environment variable support** (including .env files)
- **Separate validation:**
  - `LoadConfig()` - Full validation (requires Discord for bot)
  - `LoadConfigForMain()` - Relaxed validation (no Discord required)
