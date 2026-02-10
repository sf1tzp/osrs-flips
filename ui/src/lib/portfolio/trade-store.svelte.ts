import { browser } from '$app/environment';
import { openDB, type IDBPDatabase } from 'idb';
import type { Trade } from './types';

const DB_NAME = 'osrs-portfolio';
const DB_VERSION = 1;
const STORE_NAME = 'trades';

function getDb(): Promise<IDBPDatabase> {
	return openDB(DB_NAME, DB_VERSION, {
		upgrade(db) {
			const store = db.createObjectStore(STORE_NAME, { keyPath: 'id' });
			store.createIndex('by-item', 'itemId');
			store.createIndex('by-timestamp', 'timestamp');
		}
	});
}

class TradeStore {
	trades = $state<Trade[]>([]);
	initialized = $state(false);

	async init() {
		if (!browser || this.initialized) return;
		await this.refresh();
		this.initialized = true;
	}

	async addTrade(trade: Trade) {
		if (!browser) return;
		const db = await getDb();
		await db.put(STORE_NAME, trade);
		await this.refresh();
	}

	async deleteTrade(id: string) {
		if (!browser) return;
		const db = await getDb();
		await db.delete(STORE_NAME, id);
		await this.refresh();
	}

	async refresh() {
		if (!browser) return;
		const db = await getDb();
		const all = await db.getAll(STORE_NAME);
		this.trades = (all as Trade[]).sort((a, b) => b.timestamp - a.timestamp);
	}
}

export const tradeStore = new TradeStore();
