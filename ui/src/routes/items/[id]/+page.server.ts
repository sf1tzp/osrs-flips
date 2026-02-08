import type { PageServerLoad } from './$types';
import { db } from '$lib/server/db';
import { items } from '$lib/server/db/schema';
import { eq } from 'drizzle-orm';
import { error } from '@sveltejs/kit';
import {
	getPriceHistory,
	PRICE_HISTORY_RANGES,
	wikiIconUrl,
	type PriceHistoryRange,
} from '$lib/server/db/queries';

export const load: PageServerLoad = async ({ params, url }) => {
	const itemId = Number(params.id);
	if (Number.isNaN(itemId)) error(400, 'Invalid item ID');

	const rangeParam = url.searchParams.get('range');
	const range: PriceHistoryRange =
		rangeParam && PRICE_HISTORY_RANGES.includes(rangeParam as PriceHistoryRange)
			? (rangeParam as PriceHistoryRange)
			: '24h';

	const [[item], priceHistory] = await Promise.all([
		db.select().from(items).where(eq(items.itemId, itemId)).limit(1),
		getPriceHistory(itemId, range),
	]);

	if (!item) error(404, 'Item not found');

	return {
		item: { ...item, icon: item.icon ? wikiIconUrl(item.icon) : null },
		priceHistory,
		range,
	};
};
