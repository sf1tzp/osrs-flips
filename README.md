# osrs-flips

Configurable OldSchool RuneScape flip-finder with ollama and discord integrations.

![discord screenshot](https://i.imgur.com/ncYAnpr.png)
![example flip](https://i.imgur.com/BE16VyL.png)

## Usage

Configure jobs and filters in [config.yml](./config.yml). Jobs can be run individually via the cli (`just`), or configured to run on a schedule.

```bash
# Build and run directly
just run "job-name"

# Or build & deploy the compose file
just build up
```

### Jobs and Filter Configuration

You can define different jobs with custom filters. When a job runs, it first retrieves data from the wiki's price API (a single call for many items). Your configured price filters are applied to that initial set. The items are then sorted, and volume information is retrieved from the timeseries API (up to `limit` calls for `limit` items). Your configured volume filters are then applied, and data is sent to ollama for summary.

- Several price and volume filters are available to choose from. [config.yml](./config.yml) has more examples.
- You can provide specific model configurations for ollama to use.
- The prompt is available in [prompt.md](./prompt.md) and can be customized to suit your needs.

> Note: the use of 'insta buy' and `insta sell` terminology is kind of confusing. On the Grand Exchange, you try to buy items at a low "insta sell" price and sell at a high "insta buy" price.
> If you have suggestions to improve the configuration syntax, please submit them!


> Another Note: you can use `output.max_items` to limit the input context size depending on model/VRAM requirements. Optionally configure `model.num_ctx` to change the context size on ollama.

```yaml
# Trading analysis jobs
jobs:
  - name: "example"
    description: "Items with 4.5% margin, up to 1M in price, with at least 60 trades in the last hour"
    filters:
      # Pricing API filters
      margin_pct_min: 4.5           # Minimum profit margin percentage
      insta_sell_price_max: 1000000 # Maximum item price

      # Volume API filters
      limit: 500                    # Limit on volume API calls
      volume_1h_min: 60             # Filter by 20m/1h/24h activity

    output:
      max_items: 20                 # Limit item json passed to LLM
    model:
      num_ctx: 15000                # Tune model parameters as necessary

# Scheduling
schedules:
  - job_name: "example"
    cron: "0 */30 * * * *"         # Every 30 minutes
```

### Prompt & Data Strategy

# System Prompt

This project uses a technique called 'few shot learning' - We try to influence the output presentation by including a couple of examples in the system prompt.

We also include `signals.md` to give the model some added context relevant to this domain.

# User Prompt

We want to present the LLM a bunch of market data, in such a way that
- Reduces input context size
- Clearly communicates data in a way that the model expects as described in the system prompt.
- Uses words instead of numbers to convey certain concepts so the model doesn't have to translate them (ie 'flat' or 'sharp' trends).

After a couple of iterations I settled on a structued json format, which is built with some careful LLM aware refinements:
- No repeated URL characters - URL formatted is coerced by the prompt.
- Grouping related data segments under common keys, to avoid repeated key characters
- Avoids supplying incomplete or confusing zero-value data when encountered

Here's an example: [example-context-data.json](./example-context-data.json)


