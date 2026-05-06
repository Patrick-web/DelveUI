<script lang="ts">
  import { onMount } from "svelte";
  import {
    workspace,
    sessions,
    runTargets,
    targetsLoading,
    refreshTargets,
    startSession,
    launchTarget,
    type RunTarget,
  } from "./store";
  import { recency } from "./recency-store";
  import Icon from "./Icon.svelte";

  export let open = false;

  type Tab = "run" | "debug";
  let tab: Tab = "debug";
  let filter = "";
  let inputEl: HTMLInputElement | null = null;
  // Per-id launching state for instant click feedback (matches RunTargetsPanel
  // pattern). The id is removed in the action's finally so the row re-renders
  // either as a running session (filtered out) or clickable again on failure.
  let launching = new Set<string>();

  onMount(() => {
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape" && open) open = false;
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  });

  function fuzzy(q: string, s: string): boolean {
    if (!q) return true;
    const hay = s.toLowerCase();
    const needle = q.toLowerCase();
    let i = 0;
    for (const c of hay) {
      if (c === needle[i]) i++;
      if (i === needle.length) return true;
    }
    return false;
  }

  // Track only the rising edge of `open`. A reactive `$: if (open)` block
  // would re-run every time $runTargets or $workspace updated and clobber
  // whatever the user had typed into the filter.
  let prevOpen = false;
  $: if (open && !prevOpen) {
    prevOpen = true;
    filter = "";
    // Default to whichever side has items: prefer Debug when configs exist
    // (most common path), otherwise fall back to Run.
    const hasConfigs = ($workspace?.configs ?? []).some((c: any) => !c.disabled);
    const hasTargets = ($runTargets ?? []).length > 0;
    if (!hasConfigs && hasTargets) tab = "run";
    else tab = "debug";
    setTimeout(() => inputEl?.focus(), 0);
    // Trigger a discovery refresh on open if we have a workspace and no
    // targets cached yet — keeps the palette feeling fresh without forcing
    // a scan on every open.
    if ($workspace?.root && ($runTargets ?? []).length === 0) {
      refreshTargets().catch(() => {});
    }
  } else if (!open && prevOpen) {
    prevOpen = false;
  }

  // Pull live config + target lists; filter out entries already running.
  $: runningCfgIds = new Set(Object.values($sessions).map((s) => s.cfgId));
  $: configs = ($workspace?.configs ?? [])
    .filter((c: any) => !c.disabled && !runningCfgIds.has(c.id));
  $: targets = ($runTargets ?? []).filter((t: RunTarget) => !runningCfgIds.has(t.id));

  const KIND_ORDER = ["run", "test", "benchmark", "example", "attach"];

  function recencyCmp(rec: Record<string, number>) {
    return (a: { id: string; label: string }, b: { id: string; label: string }) => {
      const at = rec[a.id] ?? 0;
      const bt = rec[b.id] ?? 0;
      if (at !== bt) return bt - at;
      return a.label.localeCompare(b.label, undefined, { sensitivity: "base" });
    };
  }

  $: sortedConfigs = configs
    .slice()
    .sort(recencyCmp($recency))
    .filter((c: any) => fuzzy(filter, c.label));

  // Run tab groups by kind so test/run/attach stay visually separated.
  $: groupedTargets = (() => {
    const rec = $recency;
    const visible = targets.filter((t) =>
      fuzzy(filter, t.label + " " + (t.description ?? "")),
    );
    const buckets: Record<string, RunTarget[]> = {};
    for (const t of visible) (buckets[t.kind] ||= []).push(t);
    for (const k of Object.keys(buckets)) buckets[k].sort(recencyCmp(rec));
    const keys = [
      ...KIND_ORDER.filter((k) => buckets[k]),
      ...Object.keys(buckets).filter((k) => !KIND_ORDER.includes(k)),
    ];
    return keys.map((k) => ({ kind: k, items: buckets[k] }));
  })();

  function kindTitle(k: string): string {
    if (k === "run") return "Run";
    if (k === "test") return "Tests";
    if (k === "benchmark") return "Benchmarks";
    if (k === "example") return "Examples";
    if (k === "attach") return "Attach";
    return k;
  }

  async function onStartCfg(cfgId: string) {
    launching = new Set(launching).add(cfgId);
    try {
      open = false;
      await startSession(cfgId);
    } finally {
      launching = new Set([...launching].filter((x) => x !== cfgId));
    }
  }

  async function onLaunchTarget(id: string) {
    launching = new Set(launching).add(id);
    try {
      open = false;
      await launchTarget(id);
    } finally {
      launching = new Set([...launching].filter((x) => x !== id));
    }
  }

  async function onRefresh() {
    await refreshTargets();
  }
</script>

{#if open}
  <!-- svelte-ignore a11y-no-noninteractive-element-interactions -->
  <div
    class="rp-backdrop"
    role="presentation"
    on:click={() => (open = false)}
    on:keydown={(e) => e.key === "Escape" && (open = false)}
  ></div>
  <div class="rp-palette" role="dialog" aria-modal="true" on:click|stopPropagation on:keydown|stopPropagation>
    <div class="rp-tabs" role="tablist">
      <button
        class="rp-tab"
        class:active={tab === "debug"}
        role="tab"
        aria-selected={tab === "debug"}
        on:click={() => { tab = "debug"; filter = ""; }}
      >
        Debug <span class="rp-tab-count">{configs.length}</span>
      </button>
      <button
        class="rp-tab"
        class:active={tab === "run"}
        role="tab"
        aria-selected={tab === "run"}
        on:click={() => { tab = "run"; filter = ""; }}
      >
        Run <span class="rp-tab-count">{targets.length}</span>
      </button>
    </div>

    <div class="rp-search">
      <Icon icon="solar:magnifer-linear" size={12} color="var(--text-faint)" />
      <input
        bind:this={inputEl}
        bind:value={filter}
        placeholder={tab === "debug" ? "Filter debug configs…" : "Filter run targets…"}
        spellcheck="false"
      />
      {#if tab === "run"}
        <button
          class="rp-icon-btn"
          class:spinning={$targetsLoading}
          title={$targetsLoading ? "Scanning…" : "Refresh targets"}
          disabled={$targetsLoading}
          on:click={onRefresh}
        >
          <Icon icon="solar:refresh-linear" size={12} />
        </button>
      {/if}
    </div>

    <div class="rp-list">
      {#if tab === "debug"}
        {#if !$workspace?.root && !$workspace?.debugFile}
          <div class="rp-empty">Open a project first.</div>
        {:else if configs.length === 0}
          <div class="rp-empty">All configs are running, or none defined.</div>
        {:else if sortedConfigs.length === 0}
          <div class="rp-empty">No matches for &ldquo;{filter}&rdquo;.</div>
        {:else}
          {#each sortedConfigs as cfg (cfg.id)}
            {@const isLaunching = launching.has(cfg.id)}
            <button
              class="rp-item"
              disabled={isLaunching}
              on:click={() => onStartCfg(cfg.id)}
            >
              {#if isLaunching}
                <span class="rp-spin"></span>
              {:else}
                <Icon icon="solar:play-bold" size={12} color="var(--success)" />
              {/if}
              <span class="rp-item-name">{cfg.label}</span>
              {#if cfg.mode}
                <span class="rp-kind">{cfg.mode}</span>
              {/if}
            </button>
          {/each}
        {/if}
      {:else}
        {#if !$workspace?.root}
          <div class="rp-empty">Open a folder to discover run targets.</div>
        {:else if $targetsLoading && targets.length === 0}
          <div class="rp-empty"><span class="rp-spin"></span> Scanning…</div>
        {:else if groupedTargets.length === 0}
          <div class="rp-empty">
            {filter ? `No matches for "${filter}".` : "No targets discovered."}
          </div>
        {:else}
          {#each groupedTargets as group}
            <div class="rp-group-head">
              <span>{kindTitle(group.kind)}</span>
              <span class="rp-group-count">{group.items.length}</span>
            </div>
            {#each group.items as t (t.id)}
              {@const isLaunching = launching.has(t.id)}
              <button
                class="rp-item"
                disabled={isLaunching}
                title={t.description ?? ""}
                on:click={() => onLaunchTarget(t.id)}
              >
                {#if isLaunching}
                  <span class="rp-spin"></span>
                {:else}
                  <span class="rp-dot rp-dot-{t.kind}"></span>
                {/if}
                <div class="rp-item-body">
                  <span class="rp-item-name">{t.label}</span>
                  {#if t.description}
                    <span class="rp-item-sub">{t.description}</span>
                  {/if}
                </div>
                <span class="rp-kind">{t.kind}</span>
              </button>
            {/each}
          {/each}
        {/if}
      {/if}
    </div>
  </div>
{/if}

<style>
  .rp-backdrop {
    position: fixed;
    inset: 0;
    background: rgba(0, 0, 0, 0.4);
    z-index: 850;
    --wails-draggable: no-drag;
  }
  .rp-palette {
    position: fixed;
    top: 18vh;
    left: 50%;
    transform: translateX(-50%);
    width: min(560px, calc(100vw - 48px));
    max-height: 64vh;
    background: var(--bg-elevated);
    border: 1px solid var(--border);
    border-radius: 10px;
    box-shadow: 0 24px 64px rgba(0, 0, 0, 0.5);
    z-index: 851;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    --wails-draggable: no-drag;
  }

  .rp-tabs {
    display: flex;
    gap: 2px;
    padding: 6px;
    background: var(--bg-subtle);
    border-bottom: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }
  .rp-tab {
    flex: 1;
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: 6px;
    height: 28px;
    padding: 0 10px;
    background: transparent;
    border: 0;
    border-radius: 5px;
    color: var(--text-faint);
    font: inherit;
    font-size: 12px;
    font-weight: 600;
    cursor: pointer;
  }
  .rp-tab:hover { color: var(--text); background: rgba(255,255,255,0.04); }
  .rp-tab.active {
    color: var(--text);
    background: var(--bg-elevated);
    box-shadow: inset 0 0 0 1px var(--border);
  }
  .rp-tab-count {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-faint);
    font-weight: 400;
  }
  .rp-tab.active .rp-tab-count { color: var(--text-muted); }

  .rp-search {
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 8px 12px;
    border-bottom: 1px solid var(--border-subtle);
    flex-shrink: 0;
  }
  .rp-search input {
    flex: 1;
    background: transparent;
    border: 0;
    color: var(--text);
    font: inherit;
    font-size: var(--text-sm);
    outline: none;
  }
  .rp-icon-btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    width: 22px;
    height: 22px;
    background: transparent;
    border: 1px solid var(--border-subtle);
    border-radius: 4px;
    color: var(--text-faint);
    cursor: pointer;
  }
  .rp-icon-btn:hover { color: var(--text); background: rgba(255,255,255,0.04); }
  .rp-icon-btn:disabled { opacity: 0.5; cursor: default; }
  .rp-icon-btn.spinning { animation: rp-spin 1s linear infinite; }

  .rp-list {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: 6px;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }

  .rp-group-head {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 8px 10px 4px;
    color: var(--text-faint);
    font-size: 10px;
    font-weight: 700;
    text-transform: uppercase;
    letter-spacing: 0.6px;
  }
  .rp-group-count {
    background: var(--bg-subtle);
    color: var(--text-muted);
    padding: 1px 5px;
    border-radius: 7px;
    font-size: 9px;
    letter-spacing: 0;
    text-transform: none;
  }

  .rp-item {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    background: transparent;
    border: 0;
    padding: 7px 10px;
    color: var(--text);
    font: inherit;
    font-size: var(--text-sm);
    text-align: left;
    cursor: pointer;
    border-radius: 5px;
  }
  .rp-item:hover:not(:disabled) { background: rgba(255, 255, 255, 0.06); }
  .rp-item:disabled { cursor: default; opacity: 0.7; }
  .rp-item-body {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .rp-item-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .rp-item-sub {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .rp-kind {
    font-family: var(--font-mono);
    font-size: 10px;
    color: var(--text-faint);
    text-transform: uppercase;
    letter-spacing: 0.5px;
    flex-shrink: 0;
  }

  .rp-dot {
    display: inline-block;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    flex-shrink: 0;
  }
  .rp-dot-run { background: #4cc38a; }
  .rp-dot-test { background: #f5a623; }
  .rp-dot-attach { background: #b083ee; }
  .rp-dot-benchmark { background: #5ec1e4; }
  .rp-dot-example { background: #e4906c; }

  .rp-spin {
    display: inline-block;
    width: 12px;
    height: 12px;
    border: 1.5px solid var(--border);
    border-top-color: var(--accent);
    border-radius: 50%;
    animation: rp-spin 0.8s linear infinite;
    flex-shrink: 0;
  }
  @keyframes rp-spin { to { transform: rotate(360deg); } }

  .rp-empty {
    padding: 20px 12px;
    text-align: center;
    color: var(--text-faint);
    font-size: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }
</style>
