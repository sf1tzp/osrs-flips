# OSRS Trading Signals Documentation

## Background:

**1. The Stock Market - Primarily "Free Pricing" (aka "Market Pricing" or "Price Discovery")**

*   **Free Pricing (Market Pricing):** This is the most accurate description of how the stock market *ideally* works. It means prices are determined by the collective actions of buyers and sellers. There's no central authority setting prices.  The "active" price you see is simply the latest transaction.
*   **Price Discovery:** This is a process inherent in free pricing. It means the market is constantly figuring out the "true" value of an asset through supply and demand.
*   **Limit Orders:**  This is important to understand *how* people engage with free pricing in the stock market.
    *   **Buy Limit Order:**  You set a *minimum* price you're willing to pay for a stock.  It will only execute if the price reaches or falls below that level.
    *   **Sell Limit Order:** You set a *maximum* price you're willing to accept for a stock. It will only execute if the price reaches or exceeds that level.
*   **Market Order:** This is an order to buy or sell immediately at the *current* market price.

**2. Old School RuneScape (OSRS) Grand Exchange –  More Complicated: "Auction-Based Pricing" or "Offer-Based Pricing"**

*   **Not Free Pricing:** It’s *not* really free pricing in the true sense. While it has elements of it, it’s heavily influenced by users placing buy and sell *offers*, not just executing trades.
*   **Auction-Based Pricing / Offer-Based Pricing:** This is a better way to describe the Grand Exchange system. Users post *offers* at prices they're willing to buy or sell at.  The game then matches these offers.
*   **Buy Offers & Sell Offers:** The key here. People don’t just buy/sell at the current price; they *offer* to buy at a lower price or sell at a higher price.
*   **Bid/Ask Spread:**  You're essentially seeing the difference between the highest buy offer (the "bid") and the lowest sell offer (the "ask"). The "active" price is often somewhere in the middle of this spread.
*   **Price Fluctuation:** The active price isn't really fixed; it's constantly shifting based on the most recent offers.
*   **Influenced by Sentiment:** OSRS prices are also *heavily* affected by player sentiment, scarcity, and trends (e.g., a new quest release).

The OSRS Grand Exchange is a fascinating hybrid. It's *inspired* by market principles, but the user-offer system creates a unique dynamic.

---

## Available Metrics by Timeframe

### Volume Metrics
- `sell_volume_20m/1h/24h`: Volume of transactions at the target sell price
- `buy_volume_20m/1h/24h`: Volume of transactions at the target buy price

### Price Metrics
- `avg_sell_price_20m/1h/24h`: Average target sell price over timeframe
- `avg_buy_price_20m/1h/24h`: Average target buy price over timeframe
- `avg_margin_gp_20m/1h/24h`: Average profit margin over timeframe


### Trend Indicators
> Note the longer 1 week and 1 month timeframes for larger awareness
- `sell_price_trend_1h/24h/1w/1m`: Target sell price direction ('increasing'/'decreasing'/'flat')
- `buy_price_trend_1h/24h/1w/1m`: Target buy price direction ('increasing'/'decreasing'/'flat')

---

## Trading Signals & Patterns

### 1. Volume-Based Signals

#### High Volume Opportunity
**Signal**: `sell_volume_1h > X AND buy_volume_1h > X`
**Meaning**: Active two-way trading, liquid market
**Action**: Safe to trade with good fill rates
**Risk**: Low execution risk

#### Volume Imbalance
**Signal**: `sell_volume_1h >> buy_volume_1h` OR vice versa
**Meaning**: One-sided market pressure
**Action**:
- High buy volume = Strong demand, prices may rise
- High sell volume = Strong supply, prices may fall

#### Low Volume Warning
**Signal**: `sell_volume_1h < 5 AND buy_volume_1h < 5`
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
**Signal**: `sell_price_trend_1h = 'increasing' AND buy_price_trend_1h = 'increasing'`
**Meaning**: Both buy and sell prices rising
**Action**: Market moving up, but margins may compress
**Strategy**: Quick flips before margin compression

#### Bearish Convergence
**Signal**: `sell_price_trend_1h = 'decreasing' AND buy_price_trend_1h = 'decreasing'`
**Meaning**: Both buy and sell prices falling
**Action**: Market moving down, but margins may compress
**Strategy**: Wait for stabilization

#### Margin Expansion Pattern
**Signal**: `sell_price_trend_1h = 'increasing' AND buy_price_trend_1h = 'flat'`
**Meaning**: Buy price rising while sell price stable = growing margins
**Action**: Strong buy signal
**Confidence**: Very high

#### Margin Compression Pattern
**Signal**: `sell_price_trend_1h = 'flat' AND buy_price_trend_1h = 'increasing'`
**Meaning**: Sell price rising while buy price stable = shrinking margins
**Action**: Avoid or exit positions
**Risk**: Margins may disappear

### 4. Time-Based Signals

#### Recent Activity Surge
**Signal**: `sell_volume_20m > sell_volume_1h * 0.5`
**Meaning**: 20min volume is >50% of hourly volume (3x normal rate)
**Action**: Sudden market interest, investigate news/events
**Strategy**: Quick reaction trades

#### Stale Pricing
**Signal**: `max_hours_since_update > 2`
**Meaning**: No recent transactions
**Action**: Prices may be outdated
**Risk**: High slippage risk

#### Cross-Timeframe Trend Alignment
**Signal**: `sell_price_trend_1h = sell_price_trend_24h = sell_price_trend_1w`
**Meaning**: Consistent trend across multiple timeframes
**Action**: High-confidence directional trade
**Confidence**: Very high

### 5. Composite Opportunity Scores

#### Golden Opportunity
**Criteria**:
- `margin_gp > 1000`
- `sell_volume_1h > 10 AND buy_volume_1h > 10`
- `margin_gp > avg_margin_gp_1h * 1.1` (current margin 10% better than average)
- `max_hours_since_update < 0.5`

**Action**: Prime trading candidate
**Confidence**: Very high

#### Risk Warning
**Criteria**:
- `sell_volume_1h < 5 OR buy_volume_1h < 5`
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