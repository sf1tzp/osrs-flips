import { json } from '@sveltejs/kit';
import { env } from '$env/dynamic/private';
import type { RequestHandler } from './$types';

const VALID_TIMESTEPS = ['5m', '1h', '6h', '24h'] as const;
type Timestep = (typeof VALID_TIMESTEPS)[number];

export const GET: RequestHandler = async ({ params, url }) => {
	const itemId = Number(params.id);
	if (!Number.isInteger(itemId) || itemId <= 0) {
		return json({ error: 'Invalid item ID' }, { status: 400 });
	}

	const timestep = url.searchParams.get('timestep') as Timestep | null;
	if (!timestep || !VALID_TIMESTEPS.includes(timestep)) {
		return json({ error: 'Invalid timestep. Must be one of: 5m, 1h, 6h, 24h' }, { status: 400 });
	}

	const apiUrl = `https://prices.runescape.wiki/api/v1/osrs/timeseries?id=${itemId}&timestep=${timestep}`;

	const resp = await fetch(apiUrl, {
		headers: {
			'User-Agent': env.OSRS_API_USER_AGENT || 'osrs-flips'
		}
	});

	if (!resp.ok) {
		return json({ error: 'Wiki API error' }, { status: resp.status });
	}

	const body = await resp.json();

	const data = (body.data ?? []).map((d: Record<string, unknown>) => ({
		time: new Date((d.timestamp as number) * 1000).toISOString(),
		highVolume: d.highPriceVolume ?? null,
		lowVolume: d.lowPriceVolume ?? null
	}));

	return json({ data }, { headers: { 'Cache-Control': 'private, max-age=300' } });
};
