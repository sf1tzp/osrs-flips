<div class="mx-auto max-w-7xl p-4 sm:p-6">
  <h1 class="mb-6 text-2xl font-bold">FAQ — Understanding Signals</h1>

  <div class="space-y-6">
    <!-- Spread Widening Section -->
    <section id="spread-widening" class="rounded-lg border p-4">
      <h2 class="mb-3 text-lg font-semibold">Spread Widening</h2>
      <div class="space-y-2">
        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">What is Spread Widening?</summary>
          <p class="mt-2 text-sm text-muted-foreground">
            Spread widening occurs when the gap between an item's insta-buy and insta-sell price
            grows beyond its normal range. A wider spread means more profit per flip — you buy low
            via insta-sell and sell high via insta-buy, pocketing the difference minus GE tax.
          </p>
        </details>

        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">
            Why does the spread widen?
          </summary>
          <p class="mt-2 text-sm text-muted-foreground">
            Spreads widen when there's an imbalance between buyers and sellers — for example, when
            supply drops because fewer players are selling, or when demand spikes from an update.
            Low-volume items tend to have wider spreads naturally, but a sudden widening on a
            normally tight-spread item is a strong flip signal.
          </p>
        </details>
      </div>
    </section>

    <!-- Price Inversion Section -->
    <section id="price-inversion" class="rounded-lg border p-4">
      <h2 class="mb-3 text-lg font-semibold">Price Inversion</h2>
      <div class="space-y-2">
        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">What is Price Inversion?</summary>
          <p class="mt-2 text-sm text-muted-foreground">
            Price inversion happens when an item's insta-sell price temporarily drops below its
            insta-buy price in a way that creates an unusually large margin. This is a rare but
            highly profitable flip opportunity — the market is momentarily mispriced, and you can
            exploit the gap before it corrects.
          </p>
        </details>

        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">
            How should I act on a Price Inversion signal?
          </summary>
          <p class="mt-2 text-sm text-muted-foreground">
            Act quickly — price inversions tend to correct fast as other flippers notice the
            opportunity. Buy at the low insta-sell price and sell at the higher insta-buy price. Be
            mindful of buy limits and GE tax, which can eat into your margin on high-value items.
          </p>
        </details>
      </div>
    </section>

    <!-- MACD Section -->
    <section id="macd" class="rounded-lg border p-4">
      <h2 class="mb-3 text-lg font-semibold">MACD (Moving Average Convergence / Divergence)</h2>
      <div class="space-y-2">
        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">What is MACD?</summary>
          <p class="mt-2 text-sm text-muted-foreground">
            MACD compares two moving averages of an item's price — a short-term average and a
            long-term average. When the short-term average pulls ahead of the long-term average, it
            means recent prices are rising faster than the overall trend. When it falls behind,
            prices are losing steam.
          </p>
        </details>

        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">
            How does this apply to the Grand Exchange?
          </summary>
          <div class="mt-2 space-y-2 text-sm text-muted-foreground">
            <p>
              Imagine Dragon Bones have been trading around 2,500 gp. We track their price every
              hour and calculate two averages: the last 12 hours (short-term) and the last 26 hours
              (long-term).
            </p>
            <p>
              If a quest update spikes demand, the 12-hour average starts climbing above the 26-hour
              average. That gap is the MACD value — and when it crosses in the upward direction, it
              suggests buying momentum is building.
            </p>
          </div>
        </details>

        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">
            What is a "bullish crossover"?
          </summary>
          <p class="mt-2 text-sm text-muted-foreground">
            A bullish crossover happens when the fast (short-term) average crosses above the slow
            (long-term) average. It means buying pressure is picking up — the item's price has been
            rising recently and may continue. This is the signal the app detects and shows on the
            Signals page.
          </p>
        </details>

        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">
            What does the MACD signal score mean?
          </summary>
          <p class="mt-2 text-sm text-muted-foreground">
            The score reflects how strong the crossover is — specifically, how far the MACD line has
            crossed above the signal line relative to the item's price. A higher score means a more
            decisive shift in momentum, not just a tiny wobble.
          </p>
        </details>
      </div>
    </section>

    <!-- RSI Section -->
    <section id="rsi" class="rounded-lg border p-4">
      <h2 class="mb-3 text-lg font-semibold">RSI (Relative Strength Index)</h2>
      <div class="space-y-2">
        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">What is RSI?</summary>
          <p class="mt-2 text-sm text-muted-foreground">
            RSI measures how fast and how far an item's price has been dropping or rising. It
            produces a value between 0 and 100. Below 30 means the price has been falling hard and
            fast (oversold). Above 70 means it's been rising aggressively (overbought).
          </p>
        </details>

        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">
            How does this apply to the Grand Exchange?
          </summary>
          <p class="mt-2 text-sm text-muted-foreground">
            Say Abyssal Whips have been dropping steadily for 14 hours — maybe bots are dumping
            stock or players are panic selling after a nerf rumour. The RSI drops below 30, flagging
            the item as oversold. Historically, prices that drop this sharply tend to bounce back,
            making it a potential buying opportunity for flippers.
          </p>
        </details>

        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">
            What does "oversold" mean in OSRS terms?
          </summary>
          <p class="mt-2 text-sm text-muted-foreground">
            Oversold means the price has dropped further and faster than usual — often because of
            panic sellers, bots dumping inventory, or a temporary spike in supply. The price is
            likely below its "fair" value, and tends to recover once the selling pressure eases.
            Think of it as a fire sale at the GE.
          </p>
        </details>

        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">
            What does the RSI signal score mean?
          </summary>
          <p class="mt-2 text-sm text-muted-foreground">
            The score indicates how oversold the item is. A higher score means the RSI value is
            further below the oversold threshold (30), suggesting a more extreme sell-off and a
            stronger potential for a price bounce.
          </p>
        </details>
      </div>
    </section>

    <!-- General Section -->
    <section class="rounded-lg border p-4">
      <h2 class="mb-3 text-lg font-semibold">General</h2>
      <div class="space-y-2">
        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">
            How often are signals updated?
          </summary>
          <p class="mt-2 text-sm text-muted-foreground">
            Signals are recalculated every time new price data arrives from the OSRS Wiki API,
            typically every 5 minutes. Old signals expire automatically once the conditions that
            triggered them are no longer present.
          </p>
        </details>

        <details class="group rounded-md border px-4 py-3">
          <summary class="cursor-pointer font-medium select-none">
            What does the score number mean?
          </summary>
          <p class="mt-2 text-sm text-muted-foreground">
            Each signal has a score that reflects how strong or confident the signal is. Higher
            scores mean a more pronounced pattern — for MACD, a stronger crossover; for RSI, a more
            deeply oversold reading. Use the score to prioritise which signals to act on first.
          </p>
        </details>
      </div>
    </section>
  </div>
</div>
