import { createHash } from "node:crypto";
import { sql } from "./index";
import {
  DEFAULT_SOURCES,
  type PriceHistoryRange,
  type PriceHistorySource,
} from "$lib/price-history";

export {
  PRICE_HISTORY_RANGES,
  PRICE_HISTORY_SOURCES,
  SOURCE_OPTIONS,
  DEFAULT_SOURCES,
  SOURCE_LABELS,
} from "$lib/price-history";
export type { PriceHistoryRange, PriceHistorySource } from "$lib/price-history";

const WIKI_IMAGE_BASE = "https://oldschool.runescape.wiki/images";

export function wikiIconUrl(filename: string): string {
  const normalized = filename.replaceAll(" ", "_");
  const hash = createHash("md5").update(normalized).digest("hex");
  return `${WIKI_IMAGE_BASE}/${hash[0]}/${hash.slice(0, 2)}/${encodeURIComponent(normalized)}`;
}

export interface DashboardItem {
  itemId: number;
  name: string;
  icon: string | null;
  members: boolean;
  buyLimit: number | null;
  highAlch: number | null;
  highPrice: number | null;
  lowPrice: number | null;
  margin: number | null;
  marginPct: number | null;
  volume24h: number | null;
}

const RANGE_INTERVALS: Record<PriceHistoryRange, string> = {
  "1h": "1 hour",
  "6h": "6 hours",
  "24h": "24 hours",
  "7d": "7 days",
  "30d": "30 days",
};

export interface PriceHistoryPoint {
  time: string;
  highPrice: number | null;
  lowPrice: number | null;
  highVolume: number | null;
  lowVolume: number | null;
}

export async function getPriceHistory(
  itemId: number,
  range: PriceHistoryRange,
  source?: PriceHistorySource,
): Promise<PriceHistoryPoint[]> {
  const effectiveSource = source ?? DEFAULT_SOURCES[range];
  const interval = RANGE_INTERVALS[range];
  let rows;

  if (effectiveSource === "observations") {
    rows = await sql`
			SELECT
				observed_at AS time,
				high_price,
				low_price,
				NULL::bigint AS high_volume,
				NULL::bigint AS low_volume
			FROM price_observations
			WHERE item_id = ${itemId}
				AND observed_at >= NOW() - ${interval}::interval
			ORDER BY observed_at
		`;
  } else if (effectiveSource === "5m") {
    rows = await sql`
			SELECT
				bucket_start AS time,
				avg_high_price AS high_price,
				avg_low_price AS low_price,
				high_price_volume AS high_volume,
				low_price_volume AS low_volume
			FROM price_buckets_5m
			WHERE item_id = ${itemId}
				AND bucket_start >= NOW() - ${interval}::interval
			ORDER BY bucket_start
		`;
  } else if (effectiveSource === "1h") {
    rows = await sql`
			SELECT
				bucket_start AS time,
				avg_high_price AS high_price,
				avg_low_price AS low_price,
				high_price_volume AS high_volume,
				low_price_volume AS low_volume
			FROM price_buckets_1h
			WHERE item_id = ${itemId}
				AND bucket_start >= NOW() - ${interval}::interval
			ORDER BY bucket_start
		`;
  } else {
    rows = await sql`
			SELECT
				bucket_start AS time,
				avg_high_price AS high_price,
				avg_low_price AS low_price,
				high_price_volume AS high_volume,
				low_price_volume AS low_volume
			FROM price_buckets_24h
			WHERE item_id = ${itemId}
				AND bucket_start >= NOW() - ${interval}::interval
			ORDER BY bucket_start
		`;
  }

  return rows.map((r) => ({
    time: r.time instanceof Date ? r.time.toISOString() : String(r.time),
    highPrice: r.high_price as number | null,
    lowPrice: r.low_price as number | null,
    highVolume: r.high_volume != null ? Number(r.high_volume) : null,
    lowVolume: r.low_volume != null ? Number(r.low_volume) : null,
  }));
}

export interface DataGap {
  start: string;
  end: string;
  durationMs: number;
  missingCount: number;
}

export interface BucketCoverage {
  bucket: string;
  retention: string;
  oldest: string | null;
  newest: string | null;
  totalBuckets: number;
  expectedBuckets: number;
  gaps: DataGap[];
}

const BUCKET_CONFIGS = [
  {
    name: "5m",
    table: "price_buckets_5m",
    interval: "5 minutes",
    seconds: 300,
    retention: "7d",
  },
  {
    name: "1h",
    table: "price_buckets_1h",
    interval: "1 hour",
    seconds: 3600,
    retention: "1y",
  },
  {
    name: "24h",
    table: "price_buckets_24h",
    interval: "24 hours",
    seconds: 86400,
    retention: "5y",
  },
] as const;

async function getBucketCoverage(
  itemId: number,
  config: (typeof BUCKET_CONFIGS)[number],
): Promise<BucketCoverage> {
  const [summary, gapRows] = await Promise.all([
    sql.unsafe(
      `SELECT
				MIN(bucket_start) as oldest,
				MAX(bucket_start) as newest,
				COUNT(*)::int as total
			FROM ${config.table}
			WHERE item_id = $1`,
      [itemId],
    ),
    sql.unsafe(
      `WITH ordered AS (
				SELECT
					bucket_start,
					LEAD(bucket_start) OVER (ORDER BY bucket_start) as next_start
				FROM ${config.table}
				WHERE item_id = $1
			)
			SELECT
				bucket_start + $2::interval as gap_start,
				next_start as gap_end
			FROM ordered
			WHERE next_start IS NOT NULL
				AND next_start - bucket_start > $2::interval
			ORDER BY bucket_start DESC
			LIMIT 50`,
      [itemId, config.interval],
    ),
  ]);

  const row = summary[0];
  const oldest = row.oldest ? new Date(row.oldest).toISOString() : null;
  const newest = row.newest ? new Date(row.newest).toISOString() : null;
  const totalBuckets = row.total as number;

  let expectedBuckets = 0;
  if (oldest && newest) {
    const rangeMs = new Date(newest).getTime() - new Date(oldest).getTime();
    expectedBuckets = Math.floor(rangeMs / (config.seconds * 1000)) + 1;
  }

  const gaps: DataGap[] = gapRows.map((g) => {
    const start = new Date(g.gap_start).toISOString();
    const end = new Date(g.gap_end).toISOString();
    const durationMs = new Date(end).getTime() - new Date(start).getTime();
    const missingCount = Math.round(durationMs / (config.seconds * 1000));
    return { start, end, durationMs, missingCount };
  });

  return {
    bucket: config.name,
    retention: config.retention,
    oldest,
    newest,
    totalBuckets,
    expectedBuckets,
    gaps,
  };
}

export async function getItemDataCoverage(
  itemId: number,
): Promise<BucketCoverage[]> {
  return Promise.all(
    BUCKET_CONFIGS.map((config) => getBucketCoverage(itemId, config)),
  );
}

export async function getDashboardItems(): Promise<DashboardItem[]> {
  const rows = await sql`
		WITH latest_prices AS (
			SELECT DISTINCT ON (item_id)
				item_id,
				high_price,
				low_price
			FROM price_observations
			ORDER BY item_id, observed_at DESC
		),
		volume_24h AS (
			SELECT
				item_id,
				COALESCE(SUM(high_price_volume), 0) + COALESCE(SUM(low_price_volume), 0) AS total_volume
			FROM price_buckets_1h
			WHERE bucket_start >= NOW() - INTERVAL '24 hours'
			GROUP BY item_id
		)
		SELECT
			i.item_id,
			i.name,
			i.icon,
			i.members,
			i.buy_limit,
			i.high_alch,
			p.high_price,
			p.low_price,
			CASE
				WHEN p.high_price IS NOT NULL AND p.low_price IS NOT NULL
				THEN p.high_price - p.low_price - LEAST(FLOOR(p.high_price * 0.02), 5000000)
			END AS margin,
			CASE
				WHEN p.high_price IS NOT NULL AND p.low_price IS NOT NULL AND p.low_price > 0
				THEN ROUND(((p.high_price - p.low_price - LEAST(FLOOR(p.high_price * 0.02), 5000000))::numeric / p.low_price) * 100, 1)
			END AS margin_pct,
			v.total_volume
		FROM items i
		LEFT JOIN latest_prices p ON p.item_id = i.item_id
		LEFT JOIN volume_24h v ON v.item_id = i.item_id
		ORDER BY i.name
	`;

  return rows.map((r) => ({
    itemId: r.item_id as number,
    name: r.name as string,
    icon: r.icon ? wikiIconUrl(r.icon as string) : null,
    members: r.members as boolean,
    buyLimit: r.buy_limit as number | null,
    highAlch: r.high_alch as number | null,
    highPrice: r.high_price as number | null,
    lowPrice: r.low_price as number | null,
    margin: r.margin as number | null,
    marginPct: r.margin_pct !== null ? Number(r.margin_pct) : null,
    volume24h: r.total_volume != null ? Number(r.total_volume) : null,
  }));
}
