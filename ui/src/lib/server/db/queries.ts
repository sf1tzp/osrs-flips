import { createHash } from 'node:crypto';
import { sql } from './index';

const WIKI_IMAGE_BASE = 'https://oldschool.runescape.wiki/images';

function wikiIconUrl(filename: string): string {
	const normalized = filename.replaceAll(' ', '_');
	const hash = createHash('md5').update(normalized).digest('hex');
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
				THEN p.high_price - p.low_price
			END AS margin,
			CASE
				WHEN p.high_price IS NOT NULL AND p.low_price IS NOT NULL AND p.low_price > 0
				THEN ROUND(((p.high_price - p.low_price)::numeric / p.low_price) * 100, 1)
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
