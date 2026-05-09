import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';
import { getPriceHistory } from '$lib/server/db/queries';

export const GET: RequestHandler = async ({ params }) => {
	const itemId = Number(params.id);
	if (!Number.isInteger(itemId) || itemId <= 0) {
		return json({ error: 'Invalid item ID' }, { status: 400 });
	}

	const history = await getPriceHistory(itemId, '7d', '1h');

	const prices = history
		.filter((p) => p.highPrice != null)
		.map((p) => ({ time: p.time, highPrice: p.highPrice }));

	let avgDailyVolume: number | null = null;
	const totalVolume = history.reduce((sum, p) => {
		return sum + (p.highVolume ?? 0) + (p.lowVolume ?? 0);
	}, 0);
	if (totalVolume > 0) {
		avgDailyVolume = Math.round(totalVolume / 7);
	}

	return json({ prices, avgDailyVolume }, { headers: { 'Cache-Control': 'private, max-age=300' } });
};
