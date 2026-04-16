<script lang="ts">
  import { activeSessionId, sessionState } from "./store";
  import VariablesTree from "./VariablesTree.svelte";
  import PanelHeader from "./PanelHeader.svelte";

  $: stack = $activeSessionId
    ? ($sessionState[$activeSessionId]?.stack ?? [])
    : [];
  $: frameId = stack[0]?.id ?? 0;
</script>

<PanelHeader title="Variables" />

<div class="body">
  {#if $activeSessionId && frameId}
    <VariablesTree sessionId={$activeSessionId} {frameId} />
  {:else}
    <div class="empty">
      Variables appear when the program stops.<br />
      Hit a breakpoint or click <strong>Pause</strong> in the toolbar to break in.
    </div>
  {/if}
</div>

<style>
  .body {
    flex: 1;
    min-height: 0;
    overflow: hidden;
    display: flex;
    flex-direction: column;
  }
  .empty {
    padding: var(--space-3);
    color: var(--text-faint);
    font-size: var(--text-sm);
  }
</style>
