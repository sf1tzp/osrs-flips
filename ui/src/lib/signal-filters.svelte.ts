import { browser } from '$app/environment';
import { parseNumeric } from '$lib/utils';
import type { ActiveSignal } from '$lib/server/db/queries';

const STORAGE_KEY = 'signal-filters';

const DEFAULT_COLUMN_FILTERS: Record<string, { min: string; max: string }> = {
  lowPrice: { min: '', max: '' },
  highPrice: { min: '', max: '' },
  margin: { min: '', max: '' },
  volume24h: { min: '', max: '' },
  buyLimit: { min: '', max: '' }
};

function loadSaved(): {
  columnFilters: Record<string, { min: string; max: string }>;
} | null {
  if (!browser) return null;
  try {
    const raw = sessionStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      // Only restore if it has the new shape
      if (parsed.columnFilters) return parsed;
    }
  } catch {}
  return null;
}

class SignalFilters {
  #saved = loadSaved();

  columnFilters = $state<Record<string, { min: string; max: string }>>(
    this.#saved?.columnFilters ?? structuredClone(DEFAULT_COLUMN_FILTERS)
  );

  constructor() {
    if (browser) {
      $effect.root(() => {
        $effect(() => {
          sessionStorage.setItem(
            STORAGE_KEY,
            JSON.stringify({
              columnFilters: this.columnFilters
            })
          );
        });
      });
    }
  }

  get active() {
    return Object.values(this.columnFilters).some((f) => f.min !== '' || f.max !== '');
  }

  reset() {
    this.columnFilters = structuredClone(DEFAULT_COLUMN_FILTERS);
  }

  matchesSignal(s: ActiveSignal): boolean {
    return (
      this.#inRange(s.lowPrice, 'lowPrice') &&
      this.#inRange(s.highPrice, 'highPrice') &&
      this.#inRange(s.margin, 'margin') &&
      this.#inRange(s.volume24h, 'volume24h') &&
      this.#inRange(s.buyLimit, 'buyLimit')
    );
  }

  matchesItemSignals(signals: ActiveSignal[]): boolean {
    if (signals.length === 0) return false;
    const first = signals[0];
    // Volume and buy limit are the same for all signals on the item
    if (!this.#inRange(first.volume24h, 'volume24h')) return false;
    if (!this.#inRange(first.buyLimit, 'buyLimit')) return false;
    // Item matches if any of its signals match price/margin filters
    return signals.some(
      (s) =>
        this.#inRange(s.lowPrice, 'lowPrice') &&
        this.#inRange(s.highPrice, 'highPrice') &&
        this.#inRange(s.margin, 'margin')
    );
  }

  #inRange(value: number | null, key: string): boolean {
    const filter = this.columnFilters[key];
    if (!filter || (filter.min === '' && filter.max === '')) return true;
    if (value == null) return false;
    const min = parseNumeric(filter.min);
    const max = parseNumeric(filter.max);
    if (min > 0 && value < min) return false;
    if (max > 0 && value > max) return false;
    return true;
  }
}

export const signalFilters = new SignalFilters();
