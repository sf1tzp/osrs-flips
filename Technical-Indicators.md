# Technical Indicators
## Moving Price Averages
Most technical indicators are built off of Moving Averages. There are two general classes of Moving Averages:
- **Simple Moving Average:**
	Average of the last `n` days' Closing Price.
-  **Weighted / Exponential Moving Average:**
	Average of the last `n` days' Closing Price, where older days Closing Prices are reduced proportionally in the calculation. In a Weighted Moving Average, the rate of reduction is linear. In Exponential Moving Averages, the rate of reduction increases for each date in the past.
### When Averages Cross
When a short term average crosses above a long term average it indicates a bullish trend.

When a stock **price** drops below the 200-day moving average, it indicates there is resistance at that level. It typically indicates the beginning of a bearish trend.
### Limitations
- Moving Averages are a _lagging indicator_, and are based solely on past pricing information.
	- Fundamental Indicators may dramatically influence stock price.
- Each company will have unique patterns of price history and volatility so judgements based on moving averages cannot be applied universally.
- A short moving average may indicate a trend that may run counter to a larger trend indicated by a longer moving average. It's important to consider multiple moving averages for this reason.
- Weighting a Moving Average biases the indicator towards recent activity, which may make it misleading.
- Moving Indicators do not reflect cyclical patterns. If a price moves up and down frequently, moving averages are unlikely to indicate trends.
- Moving averages can be noisy, and are sometimes smoothed by additional functions. See the [[MicGinley Dynamic Formula for Moving Average Smoothing]].
## Moving Average Convergence / Divergence (MACD)
MACD is a technique of comparing a short and **Exponential** moving averages (Typically 12 and 26 days), and indicates the _momentum_ of price movement:

- Points on the MACD line is defined by `12 day average - 26 day average`.
- A 9 Day **EMA** of the MACD line is used as the **signal line**.
- Each day, the Signal line value is subtracted from the MACD value to create a histogram value.

MACD is **positive** when the 12 day average is higher than the 26 day average.

When MACD value crosses the Signal Line value, it indicates a potential reversal in stock price movement direction.

When MACD _diverges_ from the stock **price** (eg, a lower price but higher MACD compared to the previous day), it indicates a potential reversal in stock price movement direction.

## Average Directional Index (ADX)
ADX quantifies trend strength by measuring the degree of directional movement in price. It **does not** indicate the direction of movement, but the **strength of the trend** in a particular direction.

ADX is most reliable when markets are **trending**.

ADX values range between 0 and 100, representing the **strength of the trend**:
- ADX values below 20 indicate a non-trending, sideways, market.
- When ADX values rise above ~25, it indicates the formation of a trend.
- When ADX values rise above ~50, it indicates a strong trend.
- As the ADX values rise higher, it indicates a stronger trend. However, the likelihood of reversals increase when ADX is high.

ADX is typically graphed along with the +/- DI lines, to visualize the **direction of the trend**. If ADX indicates there is a trend:
- If +DI > -DI the trend is positive.
- If -DI > +DI the trend is negative.
- When +/- DI lines cross and the ADX value is strong, it indicates likely continued price movement in the direction direction.

ADX will vary over time, but a series of ADX peaks above 25 indicate _trend momentum_:
- If ADX peak values are increasing, the trend momentum is also increasing and Vice Versa.
- The price of the stock is likely to follow the trend even when the trend is losing momentum.
- When stock **price** and ADX _diverge_ (eg, Price has risen but ADX falls after a lower peak), it indicates that the trend momentum is **changing direction**.

When ADX is rising below 20, a trend still hasn't been established. However if there is **high trading volume** at the same time, it indicates a trend will likely be established.

When ADX drops below 20 and remains there, it indicates that the market will likely begin to move sideways, and the trading strategy should be adjusted.

### ADX Calculation
ADX values are derived by first calculating the positive and negative _directional movements_:
- +DM = `Current High - Previous High`
- -DM = `Previous Low - Current Low`
> If both DM are positive, the smaller value is clamped at 0 for the day.

Next, the _True Range_ value is determined by the largest of:
- `Current High - Current Low`
- `Current High - Previous Close`
- `Current Low - Previous Low`

From these, _directional indicators_ are calculated from the average of _directional movements_ and _true range_ for the time window (typically 14 days):
- +DI = `(avg +DM / avg TR) * 100`
- -DI = `(avg -DM / avg TR) * 100`

The _directional movement index_ is then calculated from the absolute difference relative to the absolute sums of directional indicators:
- DX = `(|+DI - -DI| / |+DI + -DI|) * 100`

The **Average Directional Index** is then calculated, typically using a weighted moving average of DI values.

## Relative Strength Index (RSI)
RSI measures the speed and magnitude of recent price changes. It can indicate overbought or oversold conditions, which can lead to trend reversal.

RSI is more reliable when markets are moving **sideways**.

> **Do not** confuse RSI and relative strength. The first refers to changes in the price momentum of one security. The second compares the price performance of two or more securities.

RSI values range between 0 and 100.
During sideways market movements:
- RSI values below 30 typically indicate an oversold condition.
- RSI values above 70 typically indicate an overbought condition.

RSI is most reliable in sideways markets, rather than trending markets:
- During an uptrend, RSI typically stays well above 30 and frequently hits 70.
- During a downtrend, RSI typically stays well below 70 and frequently hits 30.

In trending markets, the oversold/overbought thresholds should be adjusted.

### RSI Calculation
RSI is calculated from average Gain and Loss.
- Average Gain is `(Closing Price 1 + ... Closing Price n) / n` for all positive closing prices (`Closing Price > Opening Price`) in the window.
- Average Loss is similar but for all negative closing prices (`Closing Price < Opening Price`)
- Gain Factor = `(Previous Average Gain * n-1) + Current Gain`
- Loss Factor = `(Previous Average Loss * n-1) + Current Loss`
- RSI = `100 - (100 / (1 + (Gain Factor / Loss Factor)))`


## Stochastic Oscillator
The Stochastic Oscillator is another indicator which tracks the current closing price against recent closing prices. It's used to generate overbought and oversold signals when the Stochastic value crosses 80 and 20.

The Stochastic Oscillator is typically graphed along with the 3 day moving average of stochastic value (often seen as **%K** and **%D** or the "_fast_" and "_slow_" stochastic indicators).

Because prices are thought to follow momentum (prices close near to the high in an uptrend and close near to the low in a downtrend), the fast and slow stochastic indicator lines will cross when a reversal in trend is approaching.

## Volume Techniques

1. Relative Volume (RVOL) — the most common baseline metric. Exactly what we'd compute: current_volume / avg_volume. Standard thresholds from the literature:
- 1.5–2x = moderate confirmation
- 2–3x = strong confirmation
- 3x+ = very strong conviction
- <0.5x = weak / potential false signal

2. Volume Confirmation — used as a filter/qualifier layer, not a modifier of the underlying indicator. The indicator says "this is a signal", volume says "I believe it" or "I'm skeptical". Breakouts with >150% of 20-day avg
volume are considered confirmed; <50% are considered suspect.

3. Weighted Composite Scoring — normalize each indicator to a common scale, assign weights, sum. Volume is often treated as a separate weight rather than modifying other weights.

4. Price-Volume Divergence — when price moves but volume doesn't follow, it signals a weakening setup. The inverse (volume spike + signal alignment) signals conviction.

The key pattern across all of these: volume is a confidence layer alongside signals, not a modifier of them. The typical architecture is:

individual signals (unchanged) → composite score → volume confidence qualifier

For our case, this maps cleanly to:
- compositeScore = sum(individual_scores) — rewards signal count + strength
- volumeConfidence = RVOL mapped to a 0–1 scale using the standard thresholds
- Display and sort by both, or combine as compositeScore * volumeConfidence

This way MACD=0.8 stays 0.8, RSI=0.6 stays 0.6, and volume is a separate dimension the user can see and reason about. Want me to update the plan with this approach?

Sources:
- https://www.luxalgo.com/blog/volume-analysis-techniques-to-confirm-setups/
- https://blueberrymarkets.com/market-analysis/technical-indicator-composites-the-complete-guide/
- https://www.luxalgo.com/blog/using-volume-to-confirm-trends-best-trading-strategies/
- https://www.tradingview.com/script/kRfTB5B7-Volume-Weighted-RSI-Multi-Normalized-MACD/

