import type { PageServerLoad } from './$types';
import { db } from '$lib/server/db';
import { items } from '$lib/server/db/schema';
import { eq } from 'drizzle-orm';
import { error } from '@sveltejs/kit';

export const load: PageServerLoad = async ({ params }) => {
	const itemId = Number(params.id);
	if (Number.isNaN(itemId)) error(400, 'Invalid item ID');

	const [item] = await db.select().from(items).where(eq(items.itemId, itemId)).limit(1);
	if (!item) error(404, 'Item not found');

	return { item };
};
