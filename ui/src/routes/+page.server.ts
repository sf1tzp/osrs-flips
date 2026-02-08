import type { PageServerLoad } from './$types';
import { getDashboardItems } from '$lib/server/db/queries';

export const load: PageServerLoad = async () => {
	const items = await getDashboardItems();
	return { items };
};
