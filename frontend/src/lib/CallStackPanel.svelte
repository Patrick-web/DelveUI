<script lang="ts">
  import { activeSessionId, sessionState } from "./store";
  import PanelHeader from "./PanelHeader.svelte";

  let selectedId = 0;
  $: stack = $activeSessionId
    ? ($sessionState[$activeSessionId]?.stack ?? [])
    : [];
  $: if (stack.length && !selectedId) selectedId = stack[0].id;

  function shortPath(p: string) {
    if (!p) return "?";
    return p.split("/").slice(-2).join("/");
  }
</script>

<PanelHeader title="Call Stack" />

<div class="body">
  {#if stack.length === 0}
    <div class="empty">
      No stack frames yet.<br />
      Hit a breakpoint or click <strong>Pause</strong> in the toolbar to break into the program.
    </div>
  {/if}
  {#each stack as f}
    <button
      class="frame"
      class:sel={f.id === selectedId}
      on:click={() => (selectedId = f.id)}
    >
      <div class="fname">{f.name}</div>
      <div class="floc">{shortPath(f.source?.path ?? "")}:{f.line}</div>
    </button>
  {/each}
</div>

<style>
  .body {
    flex: 1;
    overflow: auto;
    display: flex;
    flex-direction: column;
  }
  .empty {
    padding: var(--space-3);
    color: var(--text-faint);
    font-size: var(--text-sm);
  }
  .frame {
    display: block;
    width: 100%;
    text-align: left;
    background: transparent;
    border: 0;
    padding: var(--space-1) var(--space-3);
    cursor: pointer;
    color: var(--text);
  }
  .frame:hover {
    background: var(--bg-subtle);
  }
  .frame.sel {
    background: var(--accent-subtle);
  }
  .fname {
    font-family: var(--font-mono);
    font-size: var(--text-sm);
  }
  .floc {
    font-size: var(--text-xs);
    color: var(--text-faint);
    font-family: var(--font-mono);
  }
</style>
