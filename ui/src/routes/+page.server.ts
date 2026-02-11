import type { PageServerLoad } from "./$types";
import { getDashboardItems, getActiveSignals } from "$lib/server/db/queries";

export const load: PageServerLoad = async () => {
  const [items, signals] = await Promise.all([
    getDashboardItems(),
    getActiveSignals(),
  ]);
  return { items, signals };
};
