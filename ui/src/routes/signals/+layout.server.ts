import type { LayoutServerLoad } from './$types';
import { getActiveSignals } from '$lib/server/db/queries';

export const load: LayoutServerLoad = async () => {
  const all = await getActiveSignals();
  return {
    flipSignals: all
      .filter((s) => s.signalType === 'spread_widening' || s.signalType === 'price_inversion')
      .sort((a, b) => (b.margin ?? 0) - (a.margin ?? 0)),
    momentumSignals: all.filter(
      (s) => s.signalType === 'macd_crossover' || s.signalType === 'rsi_oversold'
    )
  };
};
