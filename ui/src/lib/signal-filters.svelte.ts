import { parseNumeric } from '$lib/utils';
import type { ActiveSignal } from '$lib/server/db/queries';

class SignalFilters {
  buyPriceMin = $state('');
  buyPriceMax = $state('');
  sellPriceMin = $state('');
  sellPriceMax = $state('');
  volumeMin = $state('');
  volumeMax = $state('');

  get active() {
    return (
      this.buyPriceMin !== '' ||
      this.buyPriceMax !== '' ||
      this.sellPriceMin !== '' ||
      this.sellPriceMax !== '' ||
      this.volumeMin !== '' ||
      this.volumeMax !== ''
    );
  }

  reset() {
    this.buyPriceMin = '';
    this.buyPriceMax = '';
    this.sellPriceMin = '';
    this.sellPriceMax = '';
    this.volumeMin = '';
    this.volumeMax = '';
  }

  matchesSignal(s: ActiveSignal): boolean {
    return (
      this.#inRange(s.lowPrice, this.buyPriceMin, this.buyPriceMax) &&
      this.#inRange(s.highPrice, this.sellPriceMin, this.sellPriceMax) &&
      this.#inRange(s.volume24h, this.volumeMin, this.volumeMax)
    );
  }

  matchesItemSignals(signals: ActiveSignal[]): boolean {
    // Item matches if any of its signals match price filters,
    // and volume filter (same for all signals on the item)
    if (signals.length === 0) return false;
    const first = signals[0];
    if (!this.#inRange(first.volume24h, this.volumeMin, this.volumeMax)) return false;
    return signals.some(
      (s) =>
        this.#inRange(s.lowPrice, this.buyPriceMin, this.buyPriceMax) &&
        this.#inRange(s.highPrice, this.sellPriceMin, this.sellPriceMax)
    );
  }

  #inRange(value: number | null, minStr: string, maxStr: string): boolean {
    if (minStr === '' && maxStr === '') return true;
    if (value == null) return false;
    const min = parseNumeric(minStr);
    const max = parseNumeric(maxStr);
    if (min > 0 && value < min) return false;
    if (max > 0 && value > max) return false;
    return true;
  }
}

export const signalFilters = new SignalFilters();
