import type { LayoutServerLoad } from './$types';
import { getActiveSignals, type ActiveSignal } from '$lib/server/db/queries';

export interface CompoundedItem {
  itemId: number;
  itemName: string;
  itemIcon: string | null;
  signals: ActiveSignal[];
  compositeScore: number;
  volumeConfidence: number | null;
  highPrice: number | null;
  lowPrice: number | null;
  margin: number | null;
  buyLimit: number | null;
}

export const load: LayoutServerLoad = async () => {
  const all = await getActiveSignals();

  // Group by item to find compounded signals (2+ signals on same item)
  const byItem = new Map<number, ActiveSignal[]>();
  for (const s of all) {
    let list = byItem.get(s.itemId);
    if (!list) {
      list = [];
      byItem.set(s.itemId, list);
    }
    list.push(s);
  }

  const compoundedItems: CompoundedItem[] = [];
  for (const [itemId, signals] of byItem) {
    if (signals.length < 2) continue;
    const first = signals[0];
    const compositeScore = signals.reduce((sum, s) => sum + s.score, 0);
    // Extract volume_confidence from signal metadata (max across signals for the item)
    const volConfValues = signals
      .map((s) => s.metadata?.volume_confidence)
      .filter((v): v is number => typeof v === 'number');
    const volumeConfidence = volConfValues.length > 0 ? Math.max(...volConfValues) : null;
    // Use prices from whichever signal has them (flip signals have distinct high/low)
    const withMargin = signals.find((s) => s.margin != null);
    compoundedItems.push({
      itemId,
      itemName: first.itemName,
      itemIcon: first.itemIcon,
      signals: signals.sort((a, b) => b.score - a.score),
      compositeScore: Math.round(compositeScore * 1000) / 1000,
      volumeConfidence,
      highPrice: first.highPrice,
      lowPrice: first.lowPrice,
      margin: withMargin?.margin ?? null,
      buyLimit: first.buyLimit
    });
  }

  compoundedItems.sort(
    (a, b) =>
      b.compositeScore - a.compositeScore ||
      (b.volumeConfidence ?? -1) - (a.volumeConfidence ?? -1)
  );

  return {
    flipSignals: all
      .filter((s) => s.signalType === 'spread_widening' || s.signalType === 'price_inversion')
      .sort((a, b) => (b.margin ?? 0) - (a.margin ?? 0)),
    momentumSignals: all.filter(
      (s) => s.signalType === 'macd_crossover' || s.signalType === 'rsi_oversold'
    ),
    compoundedItems
  };
};
