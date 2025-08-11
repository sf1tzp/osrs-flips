# osrs-flips

A sophisticated system for analyzing Old School RuneScape trading opportunities, combining data analysis, LLM-powered insights, and automated Discord notifications. This project explores the intersection of financial analysis patterns and gaming markets through modern development practices.

## Why This Project?

This project emerged from several converging interests and technical curiosities:

**Crossover with Symbology**: Having spent considerable time on my other side project analyzing financial filings and market data, I found the OSRS Grand Exchange fascinating as a controlled environment that mirrors many real-world trading patterns. The structured nature of game economies provides an excellent sandbox for testing analytical approaches without real financial risk, while the underlying data patterns share surprising similarities with traditional financial markets.

**Jupyter/Pandas Data Exploration**: The OSRS API provides rich, real-time trading data that's perfect for exploratory data analysis. This project gave me an opportunity to dive deep into pandas operations, time series analysis, and data visualization techniques. The gaming context makes the exploration more engaging than traditional financial datasets, while still providing meaningful insights into market dynamics.

**Claude Sonnet 4 & Copilot Collaboration**: One of the most interesting aspects was observing how Claude Sonnet 4 and GitHub Copilot handle the challenge of converting a Python prototype into production Go code. The transition from rapid prototyping in Python to efficient, concurrent Go implementations revealed fascinating insights about AI-assisted development patterns and the strengths each tool brings to different phases of development.

**Go Deployment Patterns**: This project served as an excellent vehicle for exploring modern Go deployment strategies, including containerization, structured logging, graceful shutdown patterns, and configuration management. The real-time nature of trading data provided authentic challenges for building robust, production-ready services.

## Program Usage

### CLI vs Container Deployments

The system supports both standalone CLI execution and containerized deployment, using [just]()

**CLI Mode:**
```bash
# Build and run directly
just run

# Or run the bot component
just bot

# Or build & deploy the compose file
just build up
```

### Configuration File Syntax

The system uses a YAML configuration file (`config.yml`) to define trading jobs and analysis parameters:

```yaml
# Trading analysis jobs
jobs:
  - name: "Tempting Trades"
    description: "Looking for Margins + Recent Volume Trends, up to 3.33M buy limit"
    enabled: true
    filters:
      # Pricing API filters
      margin_pct_min: 4.5           # Minimum profit margin percentage
      insta_sell_price_max: 3330000 # Maximum item price
      insta_sell_price_min: 8000    # Minimum item price
      max_hours_since_update: 1     # Data freshness requirement
      sort_by: "margin_gp"          # Sort by absolute profit
      sort_desc: true
      limit: 500                    # Max items for volume analysis

      # Volume API filters
      volume_24h_min: 500           # Minimum 24h trade volume

    output:
      max_items: 20                 # Items passed to LLM analysis

# Job scheduling
schedules:
  - job_name: "Tempting Trades"
    cron: "0 */30 * * * *"         # Every 30 minutes
    enabled: true
```

### Prompt, Example, and Signals Integration

The system combines three key documents to create comprehensive LLM analysis:

**`prompt.md`**: Contains the main system instructions and context for the LLM, including the overall objective, output format requirements, and links to the OSRS wiki for reference.

**`signals.md`**: Provides detailed documentation of trading signals, price terminology, and market mechanics. This file explains the critical distinction between `insta_sell_price` and `insta_buy_price`, and how they relate to profitable trading strategies.

**`example-output.md`**: Demonstrates the expected format and style for analysis results, helping the LLM maintain consistency in its recommendations.

These files are mounted as volumes in the container deployment, allowing for easy updates without rebuilding the container:

```yaml
volumes:
  - ./prompt.md:/app/prompt.md:ro
  - ./signals.md:/app/signals.md:ro
  - ./example-output.md:/app/example-output.md:ro
```

#### User Prompt

We format OSRS trading data into a structured JSON array, aiming to provide concise context for the LLM without repeating __too many__ characters. It's much smaller than a completely flat table structure converted to json, allowing us to provide dozens of items to the LLM for comparison (I only have 12GB VRAM lol). The simple structure of the data is handled well by smaller models such as qwen3:4b.

For an example of our current prompt data structure see [example-context-data.json](./example-context-data.json).

---

Whether you're interested in OSRS trading, exploring AI-assisted development workflows, or learning about production Go services, this project offers a comprehensive example of how these technologies can work together to create something both useful and educational.


*Happy flipping! 🪙*

