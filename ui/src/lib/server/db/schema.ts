import {
  pgTable,
  integer,
  text,
  boolean,
  timestamp,
  bigint,
  primaryKey,
} from "drizzle-orm/pg-core";

// ── Item metadata ──────────────────────────────────────────────────────
export const items = pgTable("items", {
  itemId: integer("item_id").primaryKey(),
  name: text("name").notNull(),
  examine: text("examine"),
  members: boolean("members").default(false),
  buyLimit: integer("buy_limit"),
  highAlch: integer("high_alch"),
  lowAlch: integer("low_alch"),
  geValue: integer("ge_value"),
  icon: text("icon"),
  pollVolume: boolean("poll_volume").default(false),
  createdAt: timestamp("created_at", { withTimezone: true }).defaultNow(),
  updatedAt: timestamp("updated_at", { withTimezone: true }).defaultNow(),
});

// ── Raw price observations (hypertable, 7d retention) ──────────────────
export const priceObservations = pgTable(
  "price_observations",
  {
    itemId: integer("item_id").notNull(),
    observedAt: timestamp("observed_at", { withTimezone: true }).notNull(),
    highPrice: integer("high_price"),
    highTime: timestamp("high_time", { withTimezone: true }),
    lowPrice: integer("low_price"),
    lowTime: timestamp("low_time", { withTimezone: true }),
    ingestedAt: timestamp("ingested_at", { withTimezone: true }).defaultNow(),
  },
  (t) => [primaryKey({ columns: [t.itemId, t.observedAt] })],
);

// ── 5-minute buckets (hypertable, 7d retention) ────────────────────────
export const priceBuckets5m = pgTable(
  "price_buckets_5m",
  {
    itemId: integer("item_id").notNull(),
    bucketStart: timestamp("bucket_start", { withTimezone: true }).notNull(),
    avgHighPrice: integer("avg_high_price"),
    highPriceVolume: bigint("high_price_volume", { mode: "number" }),
    avgLowPrice: integer("avg_low_price"),
    lowPriceVolume: bigint("low_price_volume", { mode: "number" }),
    source: text("source").notNull().default("api"),
    ingestedAt: timestamp("ingested_at", { withTimezone: true }).defaultNow(),
  },
  (t) => [primaryKey({ columns: [t.itemId, t.bucketStart] })],
);

// ── 1-hour buckets (hypertable, 1y retention) ──────────────────────────
export const priceBuckets1h = pgTable(
  "price_buckets_1h",
  {
    itemId: integer("item_id").notNull(),
    bucketStart: timestamp("bucket_start", { withTimezone: true }).notNull(),
    avgHighPrice: integer("avg_high_price"),
    highPriceVolume: bigint("high_price_volume", { mode: "number" }),
    avgLowPrice: integer("avg_low_price"),
    lowPriceVolume: bigint("low_price_volume", { mode: "number" }),
    source: text("source").notNull().default("api"),
    ingestedAt: timestamp("ingested_at", { withTimezone: true }).defaultNow(),
  },
  (t) => [primaryKey({ columns: [t.itemId, t.bucketStart] })],
);

// ── 24-hour buckets (hypertable, 5y retention, compressed) ─────────────
export const priceBuckets24h = pgTable(
  "price_buckets_24h",
  {
    itemId: integer("item_id").notNull(),
    bucketStart: timestamp("bucket_start", { withTimezone: true }).notNull(),
    avgHighPrice: integer("avg_high_price"),
    highPriceVolume: bigint("high_price_volume", { mode: "number" }),
    avgLowPrice: integer("avg_low_price"),
    lowPriceVolume: bigint("low_price_volume", { mode: "number" }),
    source: text("source").notNull().default("api"),
    ingestedAt: timestamp("ingested_at", { withTimezone: true }).defaultNow(),
  },
  (t) => [primaryKey({ columns: [t.itemId, t.bucketStart] })],
);
