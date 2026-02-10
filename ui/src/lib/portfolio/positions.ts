import type { Trade, Position, PortfolioSummary } from './types';
import { calcGeTax } from './types';

interface PriceMap {
	[itemId: number]: { highPrice: number | null; lowPrice: number | null };
}

export function aggregatePositions(trades: Trade[], prices: PriceMap): Position[] {
	// Group trades by itemId
	const grouped = new Map<number, Trade[]>();
	for (const t of trades) {
		let list = grouped.get(t.itemId);
		if (!list) {
			list = [];
			grouped.set(t.itemId, list);
		}
		list.push(t);
	}

	const positions: Position[] = [];

	for (const [itemId, itemTrades] of grouped) {
		// Sort chronologically for FIFO
		const sorted = itemTrades.toSorted((a, b) => a.timestamp - b.timestamp);

		// FIFO buy lots: { quantity, pricePerUnit }
		const lots: { quantity: number; pricePerUnit: number }[] = [];
		let realizedPnl = 0;

		for (const trade of sorted) {
			if (trade.type === 'buy') {
				lots.push({ quantity: trade.quantity, pricePerUnit: trade.pricePerUnit });
			} else {
				// Sell: consume earliest buy lots (FIFO)
				let remaining = trade.quantity;
				while (remaining > 0 && lots.length > 0) {
					const lot = lots[0];
					const consumed = Math.min(remaining, lot.quantity);
					const sellRevenue = consumed * trade.pricePerUnit;
					const tax = calcGeTax(trade.pricePerUnit, itemId) * consumed;
					const costBasis = consumed * lot.pricePerUnit;
					realizedPnl += sellRevenue - tax - costBasis;

					lot.quantity -= consumed;
					remaining -= consumed;
					if (lot.quantity === 0) lots.shift();
				}
			}
		}

		// Remaining lots = current position
		const quantityHeld = lots.reduce((sum, l) => sum + l.quantity, 0);
		if (quantityHeld === 0) continue;

		const totalCost = lots.reduce((sum, l) => sum + l.quantity * l.pricePerUnit, 0);
		const avgCostBasis = totalCost / quantityHeld;

		// Use highPrice (insta-sell value) as current price for valuation
		const priceInfo = prices[itemId];
		const currentPrice = priceInfo?.highPrice ?? null;
		const currentValue = currentPrice != null ? quantityHeld * currentPrice : null;

		let unrealizedPnl: number | null = null;
		let unrealizedPnlPct: number | null = null;
		if (currentValue != null) {
			// Tax would apply if selling at current price
			const taxPerUnit = calcGeTax(currentPrice!, itemId);
			const netValue = currentValue - taxPerUnit * quantityHeld;
			unrealizedPnl = netValue - totalCost;
			unrealizedPnlPct = totalCost > 0 ? Math.round((unrealizedPnl / totalCost) * 1000) / 10 : null;
		}

		const first = sorted[0];
		positions.push({
			itemId,
			itemName: first.itemName,
			itemIcon: first.itemIcon,
			quantityHeld,
			totalCost,
			avgCostBasis: Math.round(avgCostBasis),
			currentPrice,
			currentValue,
			unrealizedPnl,
			unrealizedPnlPct
		});
	}

	// Sort by total value desc (positions with value first)
	return positions.toSorted((a, b) => (b.currentValue ?? 0) - (a.currentValue ?? 0));
}

export function computeSummary(positions: Position[]): PortfolioSummary {
	let totalValue = 0;
	let totalCost = 0;

	for (const p of positions) {
		totalCost += p.totalCost;
		if (p.currentValue != null) {
			totalValue += p.currentValue;
		}
	}

	// Account for tax on total unrealized
	let totalTax = 0;
	for (const p of positions) {
		if (p.currentPrice != null) {
			totalTax += calcGeTax(p.currentPrice, p.itemId) * p.quantityHeld;
		}
	}

	const unrealizedPnl = totalValue - totalTax - totalCost;
	const unrealizedPnlPct = totalCost > 0 ? Math.round((unrealizedPnl / totalCost) * 1000) / 10 : null;

	return { totalValue, totalCost, unrealizedPnl, unrealizedPnlPct };
}
