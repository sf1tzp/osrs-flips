export interface TradePlan {
  id: string;
  itemId: number;
  itemName: string;
  itemIcon: string | null;

  // Market snapshot at plan creation
  createdAt: number;
  snapshotInstaBuy: number | null;
  snapshotInstaSell: number | null;

  // Buy side
  quantity: number;
  buyPrice: number;

  // Lifecycle: pending → active → closed
  status: "pending" | "active" | "closed";
  filledAt: number | null;

  // Sell side (set when closing out)
  sellPrice: number | null;
  closedAt: number | null;

  notes: string;
}

export interface Position {
  itemId: number;
  itemName: string;
  itemIcon: string | null;
  quantityHeld: number;
  totalCost: number;
  avgCostBasis: number;
  currentPrice: number | null;
  currentValue: number | null;
  unrealizedPnl: number | null;
  unrealizedPnlPct: number | null;
}

export interface PortfolioSummary {
  totalValue: number;
  totalCost: number;
  unrealizedPnl: number;
  unrealizedPnlPct: number | null;
  realizedPnl: number;
}

export interface AggregationResult {
  positions: Position[];
  realizedPnl: number;
}

const BOND_ITEM_ID = 13190;

/** GE tax: 2% capped at 5M gp, except bonds which use 10% */
export function calcGeTax(sellPrice: number, itemId: number): number {
  const rate = itemId === BOND_ITEM_ID ? 0.1 : 0.02;
  const cap = itemId === BOND_ITEM_ID ? Infinity : 5_000_000;
  return Math.min(Math.floor(sellPrice * rate), cap);
}
