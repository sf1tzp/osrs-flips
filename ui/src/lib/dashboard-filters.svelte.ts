import { browser } from '$app/environment';
import type { DashboardItem } from '$lib/server/db/queries';

type SortKey = keyof DashboardItem;

const STORAGE_KEY = 'dashboard-filters';

const DEFAULT_COLUMN_FILTERS: Record<string, { min: string; max: string }> = {
	highPrice: { min: '', max: '' },
	lowPrice: { min: '', max: '' },
	margin: { min: '', max: '' },
	marginPct: { min: '', max: '' },
	buyLimit: { min: '', max: '' },
	volume24h: { min: '', max: '' }
};

function loadSaved(): {
	search: string;
	sortKey: SortKey;
	sortDir: 'asc' | 'desc';
	showTax: boolean;
	columnFilters: Record<string, { min: string; max: string }>;
} | null {
	if (!browser) return null;
	try {
		const raw = sessionStorage.getItem(STORAGE_KEY);
		if (raw) return JSON.parse(raw);
	} catch {
		// ignore parse errors and fall through to default
	}
	return null;
}

class DashboardFilters {
	#saved = loadSaved();

	search = $state(this.#saved?.search ?? '');
	sortKey = $state<SortKey>(this.#saved?.sortKey ?? 'marginPct');
	sortDir = $state<'asc' | 'desc'>(this.#saved?.sortDir ?? 'desc');
	showTax = $state(this.#saved?.showTax ?? true);
	columnFilters = $state<Record<string, { min: string; max: string }>>(
		this.#saved?.columnFilters ?? structuredClone(DEFAULT_COLUMN_FILTERS)
	);

	constructor() {
		if (browser) {
			$effect.root(() => {
				$effect(() => {
					sessionStorage.setItem(
						STORAGE_KEY,
						JSON.stringify({
							search: this.search,
							sortKey: this.sortKey,
							sortDir: this.sortDir,
							showTax: this.showTax,
							columnFilters: this.columnFilters
						})
					);
				});
			});
		}
	}

	toggleSort(key: SortKey) {
		if (this.sortKey === key) {
			this.sortDir = this.sortDir === 'asc' ? 'desc' : 'asc';
		} else {
			this.sortKey = key;
			this.sortDir = 'desc';
		}
	}

	reset() {
		this.search = '';
		this.sortKey = 'marginPct';
		this.sortDir = 'desc';
		this.showTax = true;
		this.columnFilters = structuredClone(DEFAULT_COLUMN_FILTERS);
	}
}

export const dashboardFilters = new DashboardFilters();
