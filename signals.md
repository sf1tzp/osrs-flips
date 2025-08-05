# OSRS Trading Signals Documentation

## Price Terminology (Critical Understanding)

**IMPORTANT**: The OSRS pricing terminology is counterintuitive to normal trading:

- **`sold_price`** = Price where **sell orders get filled instantly** = What you can **BUY** at
- **`bought_price`** = Price where **buy orders get filled instantly** = What you can **SELL** at
- **`margin_gp`** = `bought_price - sold_price` = Potential profit per item

### Trading Strategy
- **Buy orders**: Target the low `sold_price` (instant sell price)
- **Sell orders**: Target the high `bought_price` (instant buy price)
- **Profit**: The difference between these two prices

---

## Available Metrics by Timeframe

### Volume Metrics
- `bought_volume_20m/1h/24h`: Volume of instant-buy transactions
- `sold_volume_20m/1h/24h`: Volume of instant-sell transactions

### Price Metrics
- `avg_bought_price_20m/1h/24h`: Average instant-buy price over timeframe
- `avg_sold_price_20m/1h/24h`: Average instant-sell price over timeframe
- `avg_margin_gp_20m/1h/24h`: Average profit margin over timeframe

### Trend Indicators
- `bought_price_trend_1h/24h/1w/1m`: Price direction ('increasing'/'decreasing'/'flat')
- `sold_price_trend_1h/24h/1w/1m`: Price direction ('increasing'/'decreasing'/'flat')

---

## Trading Signals & Patterns

### 1. Volume-Based Signals

#### High Volume Opportunity
**Signal**: `bought_volume_1h > X AND sold_volume_1h > X`
**Meaning**: Active two-way trading, liquid market
**Action**: Safe to trade with good fill rates
**Risk**: Low execution risk

#### Volume Imbalance
**Signal**: `bought_volume_1h >> sold_volume_1h` OR vice versa
**Meaning**: One-sided market pressure
**Action**:
- High buy volume = Strong demand, prices may rise
- High sell volume = Strong supply, prices may fall

#### Low Volume Warning
**Signal**: `bought_volume_1h < 5 AND sold_volume_1h < 5`
**Meaning**: Illiquid market, few transactions
**Action**: Avoid or use small quantities
**Risk**: High execution risk, wide spreads

### 2. Margin-Based Signals

#### Margin Expansion
**Signal**: `margin_gp > avg_margin_gp_1h AND avg_margin_gp_1h > avg_margin_gp_24h`
**Meaning**: Profit margins are improving over time
**Action**: Good entry opportunity
**Confidence**: High if volume supports the trend

#### Margin Compression
**Signal**: `margin_gp < avg_margin_gp_1h AND avg_margin_gp_1h < avg_margin_gp_24h`
**Meaning**: Profit margins are deteriorating
**Action**: Avoid new positions, consider exiting
**Risk**: Trend may continue

#### Margin Stability
**Signal**: `abs(margin_gp - avg_margin_gp_24h) / avg_margin_gp_24h < 0.05`
**Meaning**: Consistent profit margins (< 5% variance)
**Action**: Reliable trading opportunity
**Confidence**: High for risk-averse traders

### 3. Price Trend Signals

#### Bullish Convergence
**Signal**: `bought_price_trend_1h = 'increasing' AND sold_price_trend_1h = 'increasing'`
**Meaning**: Both buy and sell prices rising
**Action**: Market moving up, but margins may compress
**Strategy**: Quick flips before margin compression

#### Bearish Convergence
**Signal**: `bought_price_trend_1h = 'decreasing' AND sold_price_trend_1h = 'decreasing'`
**Meaning**: Both buy and sell prices falling
**Action**: Market moving down, but margins may compress
**Strategy**: Wait for stabilization

#### Margin Expansion Pattern
**Signal**: `bought_price_trend_1h = 'increasing' AND sold_price_trend_1h = 'flat'`
**Meaning**: Buy price rising while sell price stable = growing margins
**Action**: Strong buy signal
**Confidence**: Very high

#### Margin Compression Pattern
**Signal**: `bought_price_trend_1h = 'flat' AND sold_price_trend_1h = 'increasing'`
**Meaning**: Sell price rising while buy price stable = shrinking margins
**Action**: Avoid or exit positions
**Risk**: Margins may disappear

### 4. Time-Based Signals

#### Recent Activity Surge
**Signal**: `bought_volume_20m > bought_volume_1h * 0.5`
**Meaning**: 20min volume is >50% of hourly volume (3x normal rate)
**Action**: Sudden market interest, investigate news/events
**Strategy**: Quick reaction trades

#### Stale Pricing
**Signal**: `max_hours_since_update > 2`
**Meaning**: No recent transactions
**Action**: Prices may be outdated
**Risk**: High slippage risk

#### Cross-Timeframe Trend Alignment
**Signal**: `bought_price_trend_1h = bought_price_trend_24h = bought_price_trend_1w`
**Meaning**: Consistent trend across multiple timeframes
**Action**: High-confidence directional trade
**Confidence**: Very high

### 5. Composite Opportunity Scores

#### Golden Opportunity
**Criteria**:
- `margin_gp > 1000`
- `bought_volume_1h > 10 AND sold_volume_1h > 10`
- `margin_gp > avg_margin_gp_1h * 1.1` (current margin 10% better than average)
- `max_hours_since_update < 0.5`

**Action**: Prime trading candidate
**Confidence**: Very high

#### Risk Warning
**Criteria**:
- `bought_volume_1h < 5 OR sold_volume_1h < 5`
- `margin_gp < avg_margin_gp_24h * 0.8` (current margin 20% worse than average)
- `max_hours_since_update > 4`

**Action**: Avoid trading
**Risk**: High

---

## Risk Management

### Position Sizing
- High volume items: Standard position size
- Low volume items: Reduce position size by 50-75%
- New/untested items: Start with minimal position

### Stop Conditions
- Margin compression > 20% from entry
- Volume drops below 50% of entry levels
- Adverse trend change across multiple timeframes

### Timing
- Best execution: High volume periods
- Avoid: Stale pricing periods (>2hrs since update)
- Monitor: 20-minute activity surges for quick opportunities