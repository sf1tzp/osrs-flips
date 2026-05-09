import { drizzle } from 'drizzle-orm/postgres-js';
import postgres from 'postgres';
import * as schema from './schema';
import { env } from '$env/dynamic/private';

let _client: ReturnType<typeof postgres> | undefined;
let _db: ReturnType<typeof drizzle> | undefined;

function getClient() {
	if (!_client) {
		if (!env.DATABASE_URL) throw new Error('DATABASE_URL is not set');
		_client = postgres(env.DATABASE_URL);
	}
	return _client;
}

export function getDb() {
	if (!_db) {
		_db = drizzle(getClient(), { schema });
	}
	return _db;
}

export const db = new Proxy({} as ReturnType<typeof drizzle>, {
	get(_, prop) {
		// eslint-disable-next-line @typescript-eslint/no-explicit-any
		return (getDb() as any)[prop];
	}
});

export const sql = new Proxy((() => undefined) as unknown as ReturnType<typeof postgres>, {
	get(_, prop) {
		// eslint-disable-next-line @typescript-eslint/no-explicit-any
		return (getClient() as any)[prop];
	},
	apply(_, __, args) {
		// eslint-disable-next-line @typescript-eslint/no-explicit-any
		return (getClient() as any)(...args);
	}
});
