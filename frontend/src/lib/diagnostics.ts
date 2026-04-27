// Lightweight runtime diagnostics for spotting UI freezes.
// Two probes:
//   1. wrapHandler() — log handlers that exceed `slowMs`.
//   2. mainThreadProbe() — log when wall-clock gap between ticks exceeds `gapMs`,
//      which indicates the JS event loop was blocked (long task / GC / native pause).

const SLOW_HANDLER_MS = 250;
const PROBE_INTERVAL_MS = 1000;
const PROBE_GAP_THRESHOLD_MS = 2000; // 1s interval → 2s+ gap means we missed a beat

export function wrapHandler<T extends (...args: any[]) => any>(
  name: string,
  fn: T,
): T {
  return (async (...args: any[]) => {
    const t0 = performance.now();
    try {
      return await fn(...args);
    } finally {
      const dt = performance.now() - t0;
      if (dt > SLOW_HANDLER_MS) {
        console.warn(`[diag] slow handler ${name}: ${dt.toFixed(0)}ms`);
      }
    }
  }) as T;
}

let probeStarted = false;
export function startMainThreadProbe() {
  if (probeStarted) return;
  probeStarted = true;
  let last = performance.now();
  setInterval(() => {
    const now = performance.now();
    const gap = now - last;
    last = now;
    if (gap > PROBE_GAP_THRESHOLD_MS) {
      console.warn(
        `[diag] main thread gap ${gap.toFixed(0)}ms (expected ~${PROBE_INTERVAL_MS}ms) — UI was blocked`,
      );
    }
  }, PROBE_INTERVAL_MS);
}
