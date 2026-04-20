<script lang="ts">
  import { activeSessionId, sessionState, selectedFrameId } from "./store";
  import PanelHeader from "./PanelHeader.svelte";

  $: stack = $activeSessionId
    ? ($sessionState[$activeSessionId]?.stack ?? [])
    : [];

  function shortPath(p: string) {
    if (!p) return "?";
    return p.split("/").slice(-2).join("/");
  }

  function selectFrame(id: number) {
    selectedFrameId.set(id);
  }

  export let hideHeader = false;
</script>

{#if !hideHeader}
<PanelHeader title="Call Stack" />
{/if}

<div class="body">
  {#if stack.length === 0}
    <div class="empty">
      No stack frames yet.<br />
      Hit a breakpoint or click <strong>Pause</strong> to break in.
    </div>
  {/if}
  {#each stack as f}
    <button
      class="frame"
      class:sel={f.id === $selectedFrameId}
      on:click={() => selectFrame(f.id)}
    >
      <div class="fname">{f.name}</div>
      <div class="floc">{shortPath(f.source?.path ?? "")}:{f.line}</div>
    </button>
  {/each}
</div>

<style>
  .body { flex:1; overflow:auto; display:flex; flex-direction:column; }
  .empty { padding:var(--space-3); color:var(--text-faint); font-size:var(--text-sm); }
  .frame { display:block; width:100%; text-align:left; background:transparent; border:0; padding:var(--space-1) var(--space-3); cursor:pointer; color:var(--text); }
  .frame:hover { background:var(--bg-subtle); }
  .frame.sel { background:var(--accent-subtle); }
  .fname { font-family:var(--font-mono); font-size:var(--text-sm); }
  .floc { font-size:var(--text-xs); color:var(--text-faint); font-family:var(--font-mono); }
</style>
