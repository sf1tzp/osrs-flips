import { browser } from "$app/environment";
import { openDB, type IDBPDatabase } from "idb";
import type { TradePlan } from "./types";

const DB_NAME = "osrs-portfolio";
const DB_VERSION = 3;
const STORE_NAME = "trades";

function getDb(): Promise<IDBPDatabase> {
  return openDB(DB_NAME, DB_VERSION, {
    upgrade(db, oldVersion) {
      if (oldVersion < 3) {
        // Fresh start: delete old store if it exists, create new one
        if (db.objectStoreNames.contains(STORE_NAME)) {
          db.deleteObjectStore(STORE_NAME);
        }
        const store = db.createObjectStore(STORE_NAME, { keyPath: "id" });
        store.createIndex("by-item", "itemId");
        store.createIndex("by-status", "status");
      }
    },
  });
}

class TradeStore {
  trades = $state<TradePlan[]>([]);
  initialized = $state(false);

  async init() {
    if (!browser || this.initialized) return;
    await this.refresh();
    this.initialized = true;
  }

  async addPlan(plan: TradePlan) {
    if (!browser) return;
    const db = await getDb();
    await db.put(STORE_NAME, plan);
    await this.refresh();
  }

  async deletePlan(id: string) {
    if (!browser) return;
    const db = await getDb();
    await db.delete(STORE_NAME, id);
    await this.refresh();
  }

  async markActive(id: string) {
    if (!browser) return;
    const db = await getDb();
    const plan = (await db.get(STORE_NAME, id)) as TradePlan | undefined;
    if (!plan || plan.status !== "pending") return;
    plan.status = "active";
    plan.filledAt = Date.now();
    await db.put(STORE_NAME, plan);
    await this.refresh();
  }

  async unfillPlan(id: string) {
    if (!browser) return;
    const db = await getDb();
    const plan = (await db.get(STORE_NAME, id)) as TradePlan | undefined;
    if (!plan || plan.status !== "active") return;
    plan.status = "pending";
    plan.filledAt = null;
    await db.put(STORE_NAME, plan);
    await this.refresh();
  }

  async updateQuantity(id: string, quantity: number) {
    if (!browser || quantity <= 0) return;
    const db = await getDb();
    const plan = (await db.get(STORE_NAME, id)) as TradePlan | undefined;
    if (!plan || plan.status === "closed") return;
    plan.quantity = quantity;
    await db.put(STORE_NAME, plan);
    await this.refresh();
  }

  async updateSellPrice(id: string, sellPrice: number) {
    if (!browser || sellPrice <= 0) return;
    const db = await getDb();
    const plan = (await db.get(STORE_NAME, id)) as TradePlan | undefined;
    if (!plan || plan.status !== "active") return;
    plan.sellPrice = sellPrice;
    await db.put(STORE_NAME, plan);
    await this.refresh();
  }

  async closePlan(id: string, sellPrice: number) {
    if (!browser) return;
    const db = await getDb();
    const plan = (await db.get(STORE_NAME, id)) as TradePlan | undefined;
    if (!plan || plan.status !== "active") return;
    plan.status = "closed";
    plan.sellPrice = sellPrice;
    plan.closedAt = Date.now();
    await db.put(STORE_NAME, plan);
    await this.refresh();
  }

  async reopenPlan(id: string) {
    if (!browser) return;
    const db = await getDb();
    const plan = (await db.get(STORE_NAME, id)) as TradePlan | undefined;
    if (!plan || plan.status !== "closed") return;
    plan.status = "active";
    plan.closedAt = null;
    await db.put(STORE_NAME, plan);
    await this.refresh();
  }

  async refresh() {
    if (!browser) return;
    const db = await getDb();
    const all = await db.getAll(STORE_NAME);
    this.trades = (all as TradePlan[]).sort(
      (a, b) => b.createdAt - a.createdAt,
    );
  }
}

export const tradeStore = new TradeStore();
