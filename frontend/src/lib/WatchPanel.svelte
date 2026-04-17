<script lang="ts">
  import { activeSessionId, activeSession, selectedFrameId, evaluate } from "./store";
  import PanelHeader from "./PanelHeader.svelte";
  import Icon from "./Icon.svelte";

  type WatchItem = {
    expr: string;
    value: string;
    type: string;
    error: string;
  };

  let watches: WatchItem[] = [];
  let newExpr = "";

  // Re-evaluate all watches when session stops (selectedFrameId changes while stopped)
  $: if ($activeSession?.state === "stopped" && $selectedFrameId && $activeSessionId) {
    refreshAll();
  }

  async function refreshAll() {
    if (!$activeSessionId || !$selectedFrameId) return;
    for (const w of watches) {
      try {
        const r = await evaluate($activeSessionId, w.expr, $selectedFrameId) as any;
        w.value = r?.result ?? "";
        w.type = r?.type ?? "";
        w.error = "";
      } catch (e: any) {
        w.value = "";
        w.type = "";
        w.error = String(e?.message ?? e);
      }
    }
    watches = watches;
  }

  function addWatch() {
    if (!newExpr.trim()) return;
    watches = [...watches, { expr: newExpr.trim(), value: "", type: "", error: "" }];
    newExpr = "";
    refreshAll();
  }

  function removeWatch(idx: number) {
    watches = watches.filter((_, i) => i !== idx);
  }

  function onKey(e: KeyboardEvent) {
    if (e.key === "Enter") addWatch();
  }
</script>

<PanelHeader title="Watch">
  <button class="btn icon" title="Refresh all" on:click={refreshAll}>
    <Icon icon="solar:refresh-linear" size={13} />
  </button>
</PanelHeader>

<div class="body">
  <div class="add-row">
    <input
      class="watch-input"
      bind:value={newExpr}
      placeholder="Add expression…"
      on:keydown={onKey}
    />
    <button class="btn icon" on:click={addWatch} title="Add">
      <Icon icon="solar:add-circle-linear" size={13} />
    </button>
  </div>

  {#if watches.length === 0}
    <div class="empty">Add expressions to watch during debugging.</div>
  {/if}

  {#each watches as w, i}
    <div class="watch-row">
      <div class="watch-expr">{w.expr}</div>
      {#if w.error}
        <div class="watch-val error">{w.error}</div>
      {:else if w.value}
        <div class="watch-val">
          <span class="val">{w.value}</span>
          {#if w.type}<span class="typ">{w.type}</span>{/if}
        </div>
      {:else}
        <div class="watch-val pending">not evaluated</div>
      {/if}
      <button class="btn icon rm" on:click={() => removeWatch(i)} title="Remove">
        <Icon icon="solar:close-circle-linear" size={12} />
      </button>
    </div>
  {/each}
</div>

<style>
  .body { flex:1; overflow:auto; }
  .add-row {
    display:flex; align-items:center; gap:var(--space-1);
    padding:var(--space-1) var(--space-2);
    border-bottom:1px solid var(--border-subtle);
  }
  .watch-input {
    flex:1; background:transparent; border:0; color:var(--text);
    font-family:var(--font-mono); font-size:var(--text-sm); outline:none;
    padding:var(--space-1) var(--space-1);
  }
  .empty { padding:var(--space-3); color:var(--text-faint); font-size:var(--text-sm); }
  .watch-row {
    display:flex; align-items:flex-start; gap:var(--space-2);
    padding:var(--space-1) var(--space-3);
    border-bottom:1px solid var(--border-subtle);
  }
  .watch-row:hover { background:var(--bg-subtle); }
  .watch-expr {
    font-family:var(--font-mono); font-size:var(--text-sm);
    color:var(--syn-fn); min-width:80px; flex-shrink:0;
  }
  .watch-val { flex:1; font-family:var(--font-mono); font-size:var(--text-sm); color:var(--text); }
  .watch-val .val { color:var(--text); }
  .watch-val .typ { color:var(--text-faint); margin-left:var(--space-2); }
  .watch-val.error { color:var(--danger); font-size:var(--text-xs); }
  .watch-val.pending { color:var(--text-faint); font-style:italic; }
  .rm { color:var(--text-faint); flex-shrink:0; }
  .rm:hover { color:var(--danger); }
</style>
