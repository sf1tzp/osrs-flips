# OSRS Flipping Analysis - AI Agent Instructions

> Always use the python provided by our VirtualEnv: ~/osrs-flipping/.venv/
> After activating the venv, always use `uv` to manage packages, etc eg `uv pip install ..., uv run ...`


## Project Overview
This codebase analyzes Old School RuneScape (OSRS) item prices for trading opportunities using the RuneScape Wiki pricing API. The core architecture revolves around a single `OSRSItemFilter` class that fetches, filters, and analyzes trading data.

## Key Architecture Patterns

### Data Flow & Price Semantics
**Critical**: OSRS pricing terminology is counterintuitive:
- `sold_price` = price where **sell orders get filled instantly** = what you can **BUY** at
- `bought_price` = price where **buy orders get filled instantly** = what you can **SELL** at
- `margin_gp` = `bought_price - sold_price` = potential profit per item

### Core Class: OSRSItemFilter
Located in `osrs_filter.py`, this is the single source of truth for all data operations:

```python
# Standard initialization pattern
item_filter = OSRSItemFilter(user_agent="learning pandas with osrs - @sf1tzp")
item_filter.load_data(force_reload=True)  # Always use force_reload for fresh data
```

### Data Loading Workflow
1. **Base Data**: `load_data()` fetches item mapping + latest prices
2. **Volume Enrichment**: `load_volume_data(max_items=N)` adds time-series metrics (expensive API calls)
3. **Filtering**: `apply_filter()` with extensive parameters for trading criteria
4. **Persistence**: Save/load with pickle format to preserve datetime objects

### Time-Based Metrics Pattern
The system calculates metrics across multiple timeframes:
- **20m, 1h, 24h**: Volume and average prices from 5-minute timeseries data
- **1w, 1m**: Trend analysis from 24-hour timeseries data
- **Trends**: `increasing`/`decreasing`/`flat` based on linear regression + 1% threshold

## Development Workflows

### API Rate Limiting
- **User-Agent Required**: RuneScape Wiki API requires descriptive User-Agent headers
- **Built-in Jitter**: `calculate_volume_metrics()` includes random delays (0.3-0.9s)
- **Batch Processing**: `load_volume_data()` processes items sequentially with progress indicators

### Data Persistence Strategy
```python
# Save filtered results (use pickle to preserve datetimes)
item_filter.save("filtered_items.pkl", format="pickle")

# Load for analysis (separate instance)
loaded_filter = OSRSItemFilter(user_agent=USER_AGENT)
loaded_filter.load_from_file("filtered_items.pkl", format="pickle")
```

### Filtering Patterns
Common filter combinations for trading analysis:
```python
# Short-term trading
item_filter.apply_filter(
    margin_min=500,
    volume_1h_min=1000,  # Both bought AND sold volume required
    max_hours_since_update=0.2,  # Recent price updates only
    sort_by=("margin_gp", "desc")
)

# Volume-based sorting
item_filter.apply_filter(sort_by=("bought_volume_1h", "desc"))
```

## Project-Specific Conventions

### Column Naming Convention
- Volume metrics: `{bought|sold}_volume_{20m|1h|24h}`
- Price averages: `avg_{bought|sold}_price_{20m|1h|24h}`
- Trends: `{bought|sold}_price_trend_{1h|24h|1w|1m}`

## Integration Points

### External Dependencies
- **Environment**: Uses `.venv` for isolated Python environment ~/osrs-flipping/.venv

- **RuneScape Wiki API**: `https://prices.runescape.wiki/api/v1/osrs/`
    - `/latest`: Current high/low prices for all items
    - `/mapping`: Item ID to name/metadata mapping
    - `/timeseries`: Historical price/volume data (5m, 1h, 6h, 24h intervals)

## Common Debugging Patterns

### Data Loading Issues
- Check User-Agent header configuration
- Verify API endpoint availability
- Use `force_reload=True` to clear cached timezone-naive data

### Volume Data Problems
- Volume metrics require separate API calls (slow for many calls, but otherwise free to use)
- Failed fetches are logged but don't break the pipeline
- Check item_id validity against mapping data

### Performance Considerations
- Jitter delays prevent API rate limiting
- Pickle format preserves complex data types efficiently

When working with this codebase, always initialize with proper User-Agent, use force_reload for fresh data, and understand the counterintuitive price semantics for trading analysis.
