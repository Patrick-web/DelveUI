<script lang="ts">
  import { sessionState, setBreakpoints, activeSessionId } from "./store";
  import PanelHeader from "./PanelHeader.svelte";
  import Icon from "./Icon.svelte";

  $: bps = $activeSessionId
    ? ($sessionState[$activeSessionId]?.breakpoints ?? {})
    : {};
  $: entries = Object.entries(bps).flatMap(([path, lines]) =>
    (lines ?? []).map((line) => ({ path, line })),
  );

  function shortPath(p: string) {
    return p.split("/").slice(-2).join("/");
  }

  async function remove(path: string, line: number) {
    if (!$activeSessionId) return;
    const current = bps[path] ?? [];
    const next = current.filter((l) => l !== line);
    await setBreakpoints($activeSessionId, path, next);
  }

  async function clearAll() {
    if (!$activeSessionId) return;
    for (const p of Object.keys(bps)) {
      await setBreakpoints($activeSessionId, p, []);
    }
  }
</script>

<PanelHeader title="Breakpoints">
  <button class="btn icon" title="Remove All" on:click={clearAll}>
    <Icon icon="solar:trash-bin-minimalistic-linear" size={13} />
  </button>
</PanelHeader>

<div class="list">
  {#if entries.length === 0}
    <div class="empty">No breakpoints</div>
  {/if}
  {#each entries as bp}
    <div class="bp" title="{bp.path}:{bp.line}">
      <Icon icon="solar:record-circle-bold" size={12} color="var(--danger)" />
      <span class="path">{shortPath(bp.path)}</span>
      <span class="line">:{bp.line}</span>
      <button
        class="btn icon x"
        title="Remove"
        on:click={() => remove(bp.path, bp.line)}
      >
        <Icon icon="solar:close-circle-linear" size={13} />
      </button>
    </div>
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
  .x:hover {
    color: var(--danger);
  }
</style>
