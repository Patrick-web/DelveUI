<script lang="ts">
  import { sessionState, setBreakpoints, activeSessionId, activeSession, globalBreakpoints, manualSourcePath } from "./store";
  import { setActivePanel } from "./panels/layout";

  function goTo(path: string, line: number) {
    manualSourcePath.set(path);
    setActivePanel("right", "source");
  }
  import PanelHeader from "./PanelHeader.svelte";
  import Icon from "./Icon.svelte";
  import * as SessionService from "../../bindings/github.com/jp/DelveUI/internal/services/sessionservice";

  let breakOnPanic = false;

  async function togglePanic() {
    breakOnPanic = !breakOnPanic;
    if (!$activeSessionId) return;
    try {
      await SessionService.SetExceptionBreakpoints($activeSessionId, breakOnPanic ? ["panic"] : []);
    } catch (e) { console.error(e); }
  }

  // Read from globalBreakpoints (works with or without a session)
  $: bps = $globalBreakpoints;
  $: entries = Object.entries(bps).flatMap(([path, lines]) =>
    (lines ?? []).map((line) => ({ path, line })),
  );

  function shortPath(p: string) {
    return p.split("/").slice(-2).join("/");
  }

  async function remove(path: string, line: number) {
    const current = bps[path] ?? [];
    const next = current.filter((l) => l !== line);
    await setBreakpoints($activeSessionId, path, next);
  }

  async function clearAll() {
    for (const p of Object.keys(bps)) {
      await setBreakpoints($activeSessionId, p, []);
    }
  }
</script>

<PanelHeader title="Breakpoints">
  <label class="panic-toggle" title="Break on Go panic">
    <input type="checkbox" checked={breakOnPanic} on:change={togglePanic} disabled={!$activeSessionId} />
    <span>panic</span>
  </label>
  <button class="btn icon" title="Remove All" on:click={clearAll}>
    <Icon icon="solar:trash-bin-minimalistic-linear" size={13} />
  </button>
</PanelHeader>

<div class="list">
  {#if entries.length === 0}
    <div class="empty">No breakpoints</div>
  {/if}
  {#each entries as bp}
    <button class="bp" title="{bp.path}:{bp.line}" on:click={() => goTo(bp.path, bp.line)}>
      <Icon icon="solar:record-circle-bold" size={12} color="var(--danger)" />
      <span class="path">{shortPath(bp.path)}</span>
      <span class="line">:{bp.line}</span>
      <button
        class="btn icon x"
        title="Remove"
        on:click|stopPropagation={() => remove(bp.path, bp.line)}
      >
        <Icon icon="solar:close-circle-linear" size={13} />
      </button>
    </button>
  {/each}
</div>

<style>
  .list {
    flex: 1;
    overflow: auto;
  }
  .empty {
    padding: var(--space-3);
    color: var(--text-faint);
    font-size: var(--text-sm);
  }
  .bp {
    display: flex;
    align-items: center;
    gap: var(--space-2);
    padding: var(--space-1) var(--space-3);
    font-size: var(--text-sm);
    font-family: var(--font-mono);
    width: 100%;
    background: transparent;
    border: 0;
    text-align: left;
    cursor: pointer;
  }
  .bp:hover {
    background: var(--bg-subtle);
  }
  .path {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--text);
  }
  .line {
    color: var(--text-faint);
  }
  .x {
    color: var(--text-faint);
  }
  .x:hover { color: var(--danger); }
  .panic-toggle {
    display:flex; align-items:center; gap:3px;
    font-size:var(--text-xs); color:var(--text-muted); cursor:pointer;
  }
  .panic-toggle input { accent-color:var(--danger); width:12px; height:12px; }
</style>
