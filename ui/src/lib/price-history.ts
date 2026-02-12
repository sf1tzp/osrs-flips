export const PRICE_HISTORY_RANGES = ['1h', '6h', '24h', '7d', '30d', '3m', '6m', '1y', '5y'] as const;
export type PriceHistoryRange = (typeof PRICE_HISTORY_RANGES)[number];

export const PRICE_HISTORY_SOURCES = ['observations', '5m', '1h', '24h'] as const;
export type PriceHistorySource = (typeof PRICE_HISTORY_SOURCES)[number];

/** Source options per range, finest to smoothest */
export const SOURCE_OPTIONS: Record<PriceHistoryRange, PriceHistorySource[]> = {
  '1h': ['observations', '5m'],
  '6h': ['observations', '5m'],
  '24h': ['observations', '5m', '1h'],
  '7d': ['observations', '5m', '1h'],
  '30d': ['1h', '24h'],
  '3m': ['1h', '24h'],
  '6m': ['1h', '24h'],
  '1y': ['1h', '24h'],
  '5y': ['24h']
};

export const DEFAULT_SOURCES: Record<PriceHistoryRange, PriceHistorySource> = {
  '1h': 'observations',
  '6h': 'observations',
  '24h': '5m',
  '7d': '1h',
  '30d': '1h',
  '3m': '1h',
  '6m': '24h',
  '1y': '24h',
  '5y': '24h'
};

export const SOURCE_LABELS: Record<PriceHistorySource, string> = {
  observations: '1min',
  '5m': '5min',
  '1h': '1hr',
  '24h': '24hr'
};
