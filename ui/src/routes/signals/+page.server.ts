import type { PageServerLoad } from "./$types";
import { getActiveSignals } from "$lib/server/db/queries";

export const load: PageServerLoad = async () => {
  const signals = await getActiveSignals();
  return { signals };
};
