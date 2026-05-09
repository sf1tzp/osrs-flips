import type { TradePlan, Position, PortfolioSummary, AggregationResult } from './types';
import { calcGeTax } from './types';

interface PriceMap {
	[itemId: number]: { highPrice: number | null; lowPrice: number | null };
}

export function aggregatePositions(plans: TradePlan[], prices: PriceMap): AggregationResult {
	// Realized P&L from closed plans
	const closedPlans = plans.filter((p) => p.status === 'closed');
	let totalRealizedPnl = 0;
	for (const p of closedPlans) {
		const tax = calcGeTax(p.sellPrice!, p.itemId);
		totalRealizedPnl += (p.sellPrice! - tax - p.buyPrice) * p.quantity;
	}

	// Active plans → positions
	const activePlans = plans.filter((p) => p.status === 'active');
	const grouped = new Map<number, TradePlan[]>();
	for (const p of activePlans) {
		let list = grouped.get(p.itemId);
		if (!list) {
			list = [];
			grouped.set(p.itemId, list);
		}
		list.push(p);
	}

	const positions: Position[] = [];

	for (const [itemId, itemPlans] of grouped) {
		const quantityHeld = itemPlans.reduce((sum, p) => sum + p.quantity, 0);
		const totalCost = itemPlans.reduce((sum, p) => sum + p.quantity * p.buyPrice, 0);
		const avgCostBasis = totalCost / quantityHeld;

		const priceInfo = prices[itemId];
		const currentPrice = priceInfo?.highPrice ?? null;
		const currentValue = currentPrice != null ? quantityHeld * currentPrice : null;

		let unrealizedPnl: number | null = null;
		let unrealizedPnlPct: number | null = null;
		if (currentValue != null) {
			const taxPerUnit = calcGeTax(currentPrice!, itemId);
			const netValue = currentValue - taxPerUnit * quantityHeld;
			unrealizedPnl = netValue - totalCost;
			unrealizedPnlPct = totalCost > 0 ? Math.round((unrealizedPnl / totalCost) * 1000) / 10 : null;
		}

		// Weighted average target sell price from plans that have one set
		const withSell = itemPlans.filter((p) => p.sellPrice != null);
		let targetSellPrice: number | null = null;
		if (withSell.length > 0) {
			const totalSellQty = withSell.reduce((s, p) => s + p.quantity, 0);
			targetSellPrice = Math.round(
				withSell.reduce((s, p) => s + p.quantity * p.sellPrice!, 0) / totalSellQty
			);
		}

		const first = itemPlans[0];
		positions.push({
			itemId,
			itemName: first.itemName,
			itemIcon: first.itemIcon,
			quantityHeld,
			totalCost,
			avgCostBasis: Math.round(avgCostBasis),
			currentPrice,
			currentValue,
			targetSellPrice,
			unrealizedPnl,
			unrealizedPnlPct
		});
	}

	return {
		positions: positions.toSorted((a, b) => (b.currentValue ?? 0) - (a.currentValue ?? 0)),
		realizedPnl: totalRealizedPnl
	};
}

export function computeSummary(positions: Position[], realizedPnl: number): PortfolioSummary {
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
	const unrealizedPnlPct =
		totalCost > 0 ? Math.round((unrealizedPnl / totalCost) * 1000) / 10 : null;

	return {
		totalValue,
		totalCost,
		unrealizedPnl,
		unrealizedPnlPct,
		realizedPnl
	};
}
